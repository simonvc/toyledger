package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/simonvc/miniledger/internal/client"
	"github.com/simonvc/miniledger/internal/ledger"
)

type jeStep int

const (
	jeStepDescription jeStep = iota
	jeStepEntryAccount
	jeStepEntryType
	jeStepEntryAmount
	jeStepEntryCurrency
	jeStepEntryMore
	jeStepConfirm
)

type entryLine struct {
	accountID string
	isDebit   bool
	amount    string // decimal like "500.00"
	currency  string
}

type accountsForJEMsg struct {
	accounts []ledger.Account
	err      error
}

type txnCreatedMsg struct {
	txn *ledger.Transaction
	err error
}

type journalEntryModel struct {
	step        jeStep
	description textinput.Model
	entries     []entryLine

	// Current entry being built
	accountInput  textinput.Model
	amountInput   textinput.Model
	isDebit       bool
	currencyIdx   int
	curOptions    []string
	moreCursor    int // 0 = add another, 1 = done

	// Account list for reference
	accounts []ledger.Account

	err       error
	done      bool
	cancelled bool
	statusMsg string
	width     int
}

func newJournalEntry() journalEntryModel {
	descInput := textinput.New()
	descInput.Placeholder = "e.g. Customer deposit"
	descInput.CharLimit = 100
	descInput.Focus()

	acctInput := textinput.New()
	acctInput.Placeholder = "e.g. 1010"
	acctInput.CharLimit = 30

	amtInput := textinput.New()
	amtInput.Placeholder = "e.g. 500.00"
	amtInput.CharLimit = 20

	return journalEntryModel{
		step:         jeStepDescription,
		description:  descInput,
		accountInput: acctInput,
		amountInput:  amtInput,
		isDebit:      true,
		curOptions:   ledger.CurrencyCodes(),
	}
}

func (m *journalEntryModel) loadAccounts(c *client.Client) tea.Cmd {
	return func() tea.Msg {
		accounts, err := c.ListAccounts(context.Background(), "", nil)
		return accountsForJEMsg{accounts: accounts, err: err}
	}
}

func (m journalEntryModel) update(msg tea.Msg, c *client.Client) (journalEntryModel, tea.Cmd) {
	switch msg := msg.(type) {
	case accountsForJEMsg:
		m.accounts = msg.accounts
		return m, nil

	case txnCreatedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.step = jeStepConfirm
			return m, nil
		}
		m.done = true
		m.statusMsg = fmt.Sprintf("Transaction %s created!", msg.txn.ID[:8])
		return m, nil

	case tea.KeyMsg:
		if key.Matches(msg, keys.Escape) {
			m.cancelled = true
			return m, nil
		}

		switch m.step {
		case jeStepDescription:
			return m.updateDescription(msg)
		case jeStepEntryAccount:
			return m.updateEntryAccount(msg)
		case jeStepEntryType:
			return m.updateEntryType(msg)
		case jeStepEntryAmount:
			return m.updateEntryAmount(msg)
		case jeStepEntryCurrency:
			return m.updateEntryCurrency(msg)
		case jeStepEntryMore:
			return m.updateEntryMore(msg)
		case jeStepConfirm:
			return m.updateConfirm(msg, c)
		}
	}
	return m, nil
}

func (m journalEntryModel) updateDescription(msg tea.KeyMsg) (journalEntryModel, tea.Cmd) {
	if key.Matches(msg, keys.Enter) {
		if m.description.Value() == "" {
			m.err = fmt.Errorf("description is required")
			return m, nil
		}
		m.err = nil
		m.step = jeStepEntryAccount
		m.accountInput.SetValue("")
		m.accountInput.Focus()
		return m, nil
	}
	var cmd tea.Cmd
	m.description, cmd = m.description.Update(msg)
	return m, cmd
}

func (m journalEntryModel) updateEntryAccount(msg tea.KeyMsg) (journalEntryModel, tea.Cmd) {
	if key.Matches(msg, keys.Enter) {
		acctID := m.accountInput.Value()
		if acctID == "" {
			m.err = fmt.Errorf("account ID is required")
			return m, nil
		}
		// Auto-detect currency from account
		for i, a := range m.accounts {
			if a.ID == acctID {
				// Find currency index
				for ci, code := range m.curOptions {
					if code == a.Currency {
						m.currencyIdx = ci
						break
					}
				}
				_ = i
				break
			}
		}
		m.err = nil
		m.step = jeStepEntryType
		m.isDebit = true
		return m, nil
	}
	var cmd tea.Cmd
	m.accountInput, cmd = m.accountInput.Update(msg)
	return m, cmd
}

func (m journalEntryModel) updateEntryType(msg tea.KeyMsg) (journalEntryModel, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Up), key.Matches(msg, keys.Down):
		m.isDebit = !m.isDebit
	case key.Matches(msg, keys.Enter):
		m.err = nil
		m.step = jeStepEntryAmount
		m.amountInput.SetValue("")
		m.amountInput.Focus()
	}
	return m, nil
}

func (m journalEntryModel) updateEntryAmount(msg tea.KeyMsg) (journalEntryModel, tea.Cmd) {
	if key.Matches(msg, keys.Enter) {
		amt := m.amountInput.Value()
		if amt == "" {
			m.err = fmt.Errorf("amount is required")
			return m, nil
		}
		// Validate it parses
		cur := m.curOptions[m.currencyIdx]
		_, err := ledger.ToMinorUnits(amt, cur)
		if err != nil {
			m.err = fmt.Errorf("invalid amount: %v", err)
			return m, nil
		}
		m.err = nil
		m.step = jeStepEntryCurrency
		return m, nil
	}
	var cmd tea.Cmd
	m.amountInput, cmd = m.amountInput.Update(msg)
	return m, cmd
}

func (m journalEntryModel) updateEntryCurrency(msg tea.KeyMsg) (journalEntryModel, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Up):
		if m.currencyIdx > 0 {
			m.currencyIdx--
		}
	case key.Matches(msg, keys.Down):
		if m.currencyIdx < len(m.curOptions)-1 {
			m.currencyIdx++
		}
	case key.Matches(msg, keys.Enter):
		// Save this entry
		m.entries = append(m.entries, entryLine{
			accountID: m.accountInput.Value(),
			isDebit:   m.isDebit,
			amount:    m.amountInput.Value(),
			currency:  m.curOptions[m.currencyIdx],
		})
		m.err = nil

		// Check if balanced and have enough entries to allow "Done"
		m.moreCursor = 0
		m.step = jeStepEntryMore
	}
	return m, nil
}

func (m journalEntryModel) updateEntryMore(msg tea.KeyMsg) (journalEntryModel, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Up), key.Matches(msg, keys.Down):
		m.moreCursor = 1 - m.moreCursor
	case key.Matches(msg, keys.Enter):
		if m.moreCursor == 0 {
			// Add another entry
			m.step = jeStepEntryAccount
			m.accountInput.SetValue("")
			m.accountInput.Focus()
			m.isDebit = true
			m.err = nil
		} else {
			// Done — validate
			if len(m.entries) < 2 {
				m.err = fmt.Errorf("need at least 2 entries")
				m.moreCursor = 0
				return m, nil
			}
			if !m.isBalanced() {
				m.err = fmt.Errorf("entries do not balance — add more entries")
				m.moreCursor = 0
				return m, nil
			}
			m.err = nil
			m.step = jeStepConfirm
		}
	}
	return m, nil
}

func (m journalEntryModel) updateConfirm(msg tea.KeyMsg, c *client.Client) (journalEntryModel, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter":
		txn := &ledger.Transaction{
			Description: m.description.Value(),
		}
		for _, e := range m.entries {
			minor, _ := ledger.ToMinorUnits(e.amount, e.currency)
			if !e.isDebit {
				minor = -minor
			}
			txn.Entries = append(txn.Entries, ledger.Entry{
				AccountID: e.accountID,
				Amount:    minor,
				Currency:  e.currency,
			})
		}
		return m, func() tea.Msg {
			created, err := c.CreateTransaction(context.Background(), txn)
			return txnCreatedMsg{txn: created, err: err}
		}
	case "n", "N":
		m.cancelled = true
	}
	return m, nil
}

func (m *journalEntryModel) isBalanced() bool {
	sums := make(map[string]int64)
	for _, e := range m.entries {
		minor, err := ledger.ToMinorUnits(e.amount, e.currency)
		if err != nil {
			return false
		}
		if !e.isDebit {
			minor = -minor
		}
		sums[e.currency] += minor
	}
	for _, s := range sums {
		if s != 0 {
			return false
		}
	}
	return true
}

func (m *journalEntryModel) balanceSummary() string {
	if len(m.entries) == 0 {
		return ""
	}
	sums := make(map[string]int64)
	var totalDebit, totalCredit int64
	for _, e := range m.entries {
		minor, _ := ledger.ToMinorUnits(e.amount, e.currency)
		if e.isDebit {
			totalDebit += minor
			sums[e.currency] += minor
		} else {
			totalCredit += minor
			sums[e.currency] -= minor
		}
	}

	var b strings.Builder
	// Use first currency for display (simplified)
	cur := m.entries[0].currency
	b.WriteString(fmt.Sprintf("  Debits:  %s %s\n", ledger.FormatAmount(totalDebit, cur), cur))
	b.WriteString(fmt.Sprintf("  Credits: %s %s\n", ledger.FormatAmount(totalCredit, cur), cur))

	if m.isBalanced() {
		b.WriteString(successStyle.Render("  BALANCED"))
	} else {
		for c, s := range sums {
			if s != 0 {
				direction := "over-debited"
				amt := s
				if s < 0 {
					direction = "over-credited"
					amt = -s
				}
				b.WriteString(errorStyle.Render(fmt.Sprintf("  UNBALANCED: %s %s %s", c, direction, ledger.FormatAmount(amt, c))))
			}
		}
	}
	return b.String()
}

func (m *journalEntryModel) view() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("New Journal Entry"))
	b.WriteString("\n\n")

	// Show existing entries so far
	if len(m.entries) > 0 {
		b.WriteString(dimStyle.Render("  Entries so far:") + "\n")
		header := fmt.Sprintf("    %-4s %-14s %12s %s", "TYPE", "ACCOUNT", "AMOUNT", "CCY")
		b.WriteString(dimStyle.Render(header) + "\n")
		for _, e := range m.entries {
			typ := "DR"
			style := debitStyle
			if !e.isDebit {
				typ = "CR"
				style = creditStyle
			}
			b.WriteString(style.Render(fmt.Sprintf("    %-4s %-14s %12s %s", typ, e.accountID, e.amount, e.currency)) + "\n")
		}
		b.WriteString("\n")
		b.WriteString(m.balanceSummary())
		b.WriteString("\n\n")
	}

	switch m.step {
	case jeStepDescription:
		b.WriteString("  Enter transaction description:\n\n")
		b.WriteString("  " + m.description.View() + "\n")

	case jeStepEntryAccount:
		entryNum := len(m.entries) + 1
		b.WriteString(fmt.Sprintf("  Entry #%d — Enter account ID:\n\n", entryNum))
		b.WriteString("  " + m.accountInput.View() + "\n")

		// Show available accounts as hint
		if len(m.accounts) > 0 {
			b.WriteString("\n" + dimStyle.Render("  Available accounts:") + "\n")
			for _, a := range m.accounts {
				name := a.Name
				if len(name) > 24 {
					name = name[:24] + ".."
				}
				b.WriteString(dimStyle.Render(fmt.Sprintf("    %-12s %s (%s)", a.ID, name, a.Currency)) + "\n")
			}
		}

	case jeStepEntryType:
		b.WriteString(fmt.Sprintf("  Account: %s\n", m.accountInput.Value()))
		b.WriteString("  Select entry type:\n\n")
		if m.isDebit {
			b.WriteString(selectedStyle.Render("  > Debit (DR)") + "\n")
			b.WriteString("    Credit (CR)\n")
		} else {
			b.WriteString("    Debit (DR)\n")
			b.WriteString(selectedStyle.Render("  > Credit (CR)") + "\n")
		}

	case jeStepEntryAmount:
		typ := "Debit"
		if !m.isDebit {
			typ = "Credit"
		}
		b.WriteString(fmt.Sprintf("  Account: %s | Type: %s\n", m.accountInput.Value(), typ))
		b.WriteString("  Enter amount (e.g. 500.00):\n\n")
		b.WriteString("  " + m.amountInput.View() + "\n")

	case jeStepEntryCurrency:
		b.WriteString(fmt.Sprintf("  Account: %s | Amount: %s\n", m.accountInput.Value(), m.amountInput.Value()))
		b.WriteString("  Select currency:\n\n")

		start := m.currencyIdx - 3
		if start < 0 {
			start = 0
		}
		end := start + 7
		if end > len(m.curOptions) {
			end = len(m.curOptions)
		}

		for i := start; i < end; i++ {
			code := m.curOptions[i]
			cur := ledger.Currencies[code]
			label := fmt.Sprintf("%s - %s", code, cur.Name)
			if i == m.currencyIdx {
				b.WriteString(selectedStyle.Render("  > "+label) + "\n")
			} else {
				b.WriteString(fmt.Sprintf("    %s\n", label))
			}
		}

	case jeStepEntryMore:
		options := []string{"Add another entry", "Done — review and submit"}
		if len(m.entries) < 2 {
			options[1] = "Done (need at least 2 entries)"
		} else if !m.isBalanced() {
			options[1] = "Done (entries must balance first)"
		}

		b.WriteString("  What next?\n\n")
		for i, opt := range options {
			if i == m.moreCursor {
				b.WriteString(selectedStyle.Render("  > "+opt) + "\n")
			} else {
				b.WriteString(fmt.Sprintf("    %s\n", opt))
			}
		}

	case jeStepConfirm:
		b.WriteString("  Review journal entry:\n\n")

		var summary strings.Builder
		summary.WriteString(fmt.Sprintf("%s %s\n\n", labelStyle.Render("Description:"), m.description.Value()))
		summary.WriteString(fmt.Sprintf("%-4s %-14s %12s %s\n", "TYPE", "ACCOUNT", "AMOUNT", "CCY"))
		summary.WriteString(fmt.Sprintf("%-4s %-14s %12s %s\n", "----", "-------", "------", "---"))
		for _, e := range m.entries {
			typ := "DR"
			if !e.isDebit {
				typ = "CR"
			}
			summary.WriteString(fmt.Sprintf("%-4s %-14s %12s %s\n", typ, e.accountID, e.amount, e.currency))
		}

		b.WriteString(boxStyle.Render(summary.String()))
		b.WriteString("\n\n")
		b.WriteString("  Post this transaction? (y/n)\n")
	}

	if m.err != nil {
		b.WriteString("\n" + errorStyle.Render("  Error: "+m.err.Error()) + "\n")
	}

	b.WriteString("\n" + dimStyle.Render("  ESC to cancel"))
	return b.String()
}
