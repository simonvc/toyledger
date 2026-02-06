package tui

import (
	"context"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/simonvc/miniledger/internal/client"
)

type mode int

const (
	modeAccountList mode = iota
	modeAccountDetail
	modeTransactionList
	modeTransactionDetail
	modeBalanceSheet
	modeWizard
	modeJournalEntry
	modeLearn
	modeRatios
)

var tabModes = []mode{modeAccountList, modeTransactionList, modeBalanceSheet, modeRatios, modeLearn}

func tabLabel(m mode) string {
	switch m {
	case modeAccountList:
		return "Accounts"
	case modeTransactionList:
		return "Transactions"
	case modeBalanceSheet:
		return "Balance Sheet"
	case modeRatios:
		return "Ratios"
	case modeLearn:
		return "Learn"
	default:
		return ""
	}
}

type App struct {
	client        *client.Client
	mode          mode
	tabIndex      int
	width, height int
	err           error
	statusMsg     string

	accountList   accountListModel
	accountDetail accountDetailModel
	txnList       txnListModel
	txnDetail     txnDetailModel
	balanceSheet  balanceSheetModel
	wizard        wizardModel
	journalEntry  journalEntryModel
	ratios        ratiosModel
	learn         learnModel
}

func NewApp(c *client.Client) *App {
	app := &App{
		client:   c,
		mode:     modeAccountList,
		tabIndex: 0,
	}
	app.learn.init()
	return app
}

func (a *App) Init() tea.Cmd {
	return tea.Batch(
		a.accountList.init(a.client),
		a.txnList.init(a.client),
		a.balanceSheet.init(a.client),
		a.ratios.init(a.client),
	)
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.accountList.width = msg.Width
		a.accountList.height = msg.Height - 6
		a.txnList.width = msg.Width
		a.txnList.height = msg.Height - 6
		a.balanceSheet.width = msg.Width
		a.balanceSheet.height = msg.Height - 6
		a.accountDetail.width = msg.Width
		a.txnDetail.width = msg.Width
		a.wizard.width = msg.Width
		a.journalEntry.width = msg.Width
		a.ratios.width = msg.Width
		a.ratios.height = msg.Height - 6
		a.learn.width = msg.Width
		a.learn.height = msg.Height - 6
		return a, nil

	}

	// Route data-loaded messages to the correct sub-model regardless of active mode.
	// This is needed because Init() fires all three loads concurrently but the bottom
	// delegation only routes to the active mode's model.
	switch typedMsg := msg.(type) {
	case accountsLoadedMsg:
		var cmd tea.Cmd
		a.accountList, cmd = a.accountList.update(msg)
		return a, cmd
	case txnsLoadedMsg:
		var cmd tea.Cmd
		a.txnList, cmd = a.txnList.update(msg)
		return a, cmd
	case balanceSheetLoadedMsg:
		var cmd tea.Cmd
		a.balanceSheet, cmd = a.balanceSheet.update(msg)
		return a, cmd
	case accountDetailLoadedMsg:
		var cmd tea.Cmd
		a.accountDetail, cmd = a.accountDetail.update(msg)
		return a, cmd
	case txnDetailLoadedMsg:
		var cmd tea.Cmd
		a.txnDetail, cmd = a.txnDetail.update(msg)
		return a, cmd
	case accountDeleteConfirmedMsg:
		id := typedMsg.id
		return a, func() tea.Msg {
			err := a.client.DeleteAccount(context.Background(), id)
			return accountDeletedMsg{id: id, err: err}
		}
	case accountDeletedMsg:
		if typedMsg.err != nil {
			a.accountList, _ = a.accountList.update(msg)
			return a, nil
		}
		a.statusMsg = "Account " + typedMsg.id + " deleted"
		return a, tea.Batch(
			a.accountList.init(a.client),
			a.balanceSheet.init(a.client),
		)
	case accountRenameRequestMsg:
		id := typedMsg.id
		name := typedMsg.name
		return a, func() tea.Msg {
			_, err := a.client.RenameAccount(context.Background(), id, name)
			return accountRenamedMsg{id: id, err: err}
		}
	case accountRenamedMsg:
		a.accountList, _ = a.accountList.update(msg)
		if typedMsg.err != nil {
			return a, nil
		}
		a.statusMsg = "Account " + typedMsg.id + " renamed"
		return a, tea.Batch(
			a.accountList.init(a.client),
			a.balanceSheet.init(a.client),
		)
	case ratiosLoadedMsg:
		var cmd tea.Cmd
		a.ratios, cmd = a.ratios.update(msg)
		return a, cmd
	case learnTxnCreatedMsg:
		var cmd tea.Cmd
		a.learn, cmd = a.learn.update(msg, a.client)
		return a, cmd
	case learnAccountsLoadedMsg:
		var cmd tea.Cmd
		a.learn, cmd = a.learn.update(msg, a.client)
		return a, cmd
	case learnRatiosLoadedMsg:
		var cmd tea.Cmd
		a.learn, cmd = a.learn.update(msg, a.client)
		return a, cmd
	case jeRatiosLoadedMsg:
		var cmd tea.Cmd
		a.journalEntry, cmd = a.journalEntry.update(msg, a.client)
		return a, cmd
	}

	// Modal modes: delegate ALL message types (not just keys)
	if a.mode == modeWizard {
		var cmd tea.Cmd
		a.wizard, cmd = a.wizard.update(msg, a.client)
		if a.wizard.done {
			a.mode = modeAccountList
			a.statusMsg = a.wizard.statusMsg
			return a, a.accountList.init(a.client)
		}
		if a.wizard.cancelled {
			a.mode = modeAccountList
			a.statusMsg = "Account creation cancelled"
		}
		return a, cmd
	}

	if a.mode == modeJournalEntry {
		var cmd tea.Cmd
		a.journalEntry, cmd = a.journalEntry.update(msg, a.client)
		if a.journalEntry.done {
			a.mode = modeTransactionList
			a.statusMsg = a.journalEntry.statusMsg
			return a, a.txnList.init(a.client)
		}
		if a.journalEntry.cancelled {
			a.mode = modeTransactionList
			a.statusMsg = "Transaction cancelled"
		}
		return a, cmd
	}

	// When account list has inline input (rename/delete confirm), delegate all keys directly
	if a.mode == modeAccountList && (a.accountList.renaming || a.accountList.confirmDelete) {
		var cmd tea.Cmd
		a.accountList, cmd = a.accountList.update(msg)
		return a, cmd
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Quit):
			return a, tea.Quit

		case key.Matches(msg, keys.Tab):
			a.tabIndex = (a.tabIndex + 1) % len(tabModes)
			a.mode = tabModes[a.tabIndex]
			a.statusMsg = ""
			return a, a.refreshTab()

		case key.Matches(msg, keys.ShiftTab):
			a.tabIndex = (a.tabIndex - 1 + len(tabModes)) % len(tabModes)
			a.mode = tabModes[a.tabIndex]
			a.statusMsg = ""
			return a, a.refreshTab()

		case key.Matches(msg, keys.Escape):
			switch a.mode {
			case modeAccountDetail:
				a.mode = modeAccountList
			case modeTransactionDetail:
				a.mode = modeTransactionList
			}
			return a, nil

		case key.Matches(msg, keys.New):
			if a.mode == modeAccountList {
				a.mode = modeWizard
				a.wizard = newWizard()
				return a, nil
			}

		case key.Matches(msg, keys.NewTxn):
			if a.mode == modeTransactionList {
				a.mode = modeJournalEntry
				a.journalEntry = newJournalEntry()
				return a, a.journalEntry.loadAccounts(a.client)
			}

		case key.Matches(msg, keys.Enter):
			switch a.mode {
			case modeAccountList:
				if acctID := a.accountList.selectedID(); acctID != "" {
					a.mode = modeAccountDetail
					return a, a.accountDetail.init(a.client, acctID)
				}
				return a, nil
			case modeTransactionList:
				if txnID := a.txnList.selectedID(); txnID != "" {
					a.mode = modeTransactionDetail
					return a, a.txnDetail.init(a.client, txnID)
				}
				return a, nil
			}
		}
	}

	// Delegate update to active sub-model
	var cmd tea.Cmd
	switch a.mode {
	case modeAccountList:
		a.accountList, cmd = a.accountList.update(msg)
	case modeAccountDetail:
		a.accountDetail, cmd = a.accountDetail.update(msg)
	case modeTransactionList:
		a.txnList, cmd = a.txnList.update(msg)
	case modeTransactionDetail:
		a.txnDetail, cmd = a.txnDetail.update(msg)
	case modeBalanceSheet:
		a.balanceSheet, cmd = a.balanceSheet.update(msg)
	case modeRatios:
		a.ratios, cmd = a.ratios.update(msg)
	case modeLearn:
		a.learn, cmd = a.learn.update(msg, a.client)
	}
	return a, cmd
}

func (a *App) refreshTab() tea.Cmd {
	switch a.mode {
	case modeAccountList:
		return a.accountList.init(a.client)
	case modeTransactionList:
		return a.txnList.init(a.client)
	case modeBalanceSheet:
		return a.balanceSheet.init(a.client)
	case modeRatios:
		return a.ratios.init(a.client)
	}
	return nil
}

func (a *App) View() string {
	// Tab bar
	tabs := ""
	for i, m := range tabModes {
		label := tabLabel(m)
		if i == a.tabIndex && a.mode != modeWizard && a.mode != modeJournalEntry {
			tabs += activeTabStyle.Render(label)
		} else {
			tabs += inactiveTabStyle.Render(label)
		}
		if i < len(tabModes)-1 {
			tabs += " "
		}
	}

	// Content
	var content string
	switch a.mode {
	case modeAccountList:
		content = a.accountList.view()
	case modeAccountDetail:
		content = a.accountDetail.view()
	case modeTransactionList:
		content = a.txnList.view()
	case modeTransactionDetail:
		content = a.txnDetail.view()
	case modeBalanceSheet:
		content = a.balanceSheet.view()
	case modeWizard:
		content = a.wizard.view()
	case modeJournalEntry:
		content = a.journalEntry.view()
	case modeRatios:
		content = a.ratios.view()
	case modeLearn:
		content = a.learn.view()
	}

	// Status bar
	status := ""
	if a.statusMsg != "" {
		status = successStyle.Render(a.statusMsg)
	}
	if a.err != nil {
		status = errorStyle.Render(a.err.Error())
	}

	helpText := dimStyle.Render("tab:switch  enter:select  esc:back  n:new  d:delete  r:rename  t:new txn  q:quit")

	return lipgloss.JoinVertical(lipgloss.Left,
		tabs,
		"",
		content,
		"",
		status,
		helpText,
	)
}
