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

type accountsLoadedMsg struct {
	accounts []ledger.Account
	balances map[string]*client.BalanceResponse
	err      error
}

// accountDeleteConfirmedMsg is sent when the user confirms deletion in the TUI.
type accountDeleteConfirmedMsg struct {
	id string
}

// accountDeletedMsg is sent after the server processes the delete.
type accountDeletedMsg struct {
	id  string
	err error
}

// accountRenameRequestMsg is sent when the user submits a new name.
type accountRenameRequestMsg struct {
	id   string
	name string
}

// accountRenamedMsg is sent after the server processes the rename.
type accountRenamedMsg struct {
	id  string
	err error
}

type accountListModel struct {
	accounts       []ledger.Account
	balances       map[string]*client.BalanceResponse
	cursor         int
	loading        bool
	err            error
	width          int
	height         int
	confirmDelete  bool
	deleteTargetID string
	renaming       bool
	renameTargetID string
	renameInput    textinput.Model
}

func (m *accountListModel) init(c *client.Client) tea.Cmd {
	m.loading = true
	return func() tea.Msg {
		accounts, err := c.ListAccounts(context.Background(), "", nil)
		if err != nil {
			return accountsLoadedMsg{err: err}
		}
		balances := make(map[string]*client.BalanceResponse, len(accounts))
		for _, a := range accounts {
			bal, err := c.GetAccountBalance(context.Background(), a.ID)
			if err == nil {
				balances[a.ID] = bal
			}
		}
		return accountsLoadedMsg{accounts: accounts, balances: balances}
	}
}

func (m accountListModel) update(msg tea.Msg) (accountListModel, tea.Cmd) {
	switch msg := msg.(type) {
	case accountsLoadedMsg:
		m.loading = false
		m.accounts = msg.accounts
		m.balances = msg.balances
		m.err = msg.err

	case accountDeletedMsg:
		m.confirmDelete = false
		m.deleteTargetID = ""
		if msg.err != nil {
			m.err = msg.err
		}

	case accountRenamedMsg:
		m.renaming = false
		m.renameTargetID = ""
		if msg.err != nil {
			m.err = msg.err
		}

	case tea.KeyMsg:
		if m.renaming {
			switch {
			case key.Matches(msg, keys.Enter):
				newName := strings.TrimSpace(m.renameInput.Value())
				if newName == "" {
					m.err = fmt.Errorf("name cannot be empty")
					return m, nil
				}
				m.err = nil
				id := m.renameTargetID
				return m, func() tea.Msg {
					return accountRenameRequestMsg{id: id, name: newName}
				}
			case key.Matches(msg, keys.Escape):
				m.renaming = false
				m.renameTargetID = ""
				m.err = nil
				return m, nil
			default:
				var cmd tea.Cmd
				m.renameInput, cmd = m.renameInput.Update(msg)
				return m, cmd
			}
		}

		if m.confirmDelete {
			switch msg.String() {
			case "y", "Y":
				id := m.deleteTargetID
				m.confirmDelete = false
				return m, func() tea.Msg {
					return accountDeleteConfirmedMsg{id: id}
				}
			default:
				m.confirmDelete = false
				m.deleteTargetID = ""
			}
			return m, nil
		}

		switch {
		case key.Matches(msg, keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, keys.Down):
			if m.cursor < len(m.accounts)-1 {
				m.cursor++
			}
		case key.Matches(msg, keys.Delete):
			if id := m.selectedID(); id != "" {
				m.confirmDelete = true
				m.deleteTargetID = id
				m.err = nil
			}
		case key.Matches(msg, keys.Rename):
			if idx := m.cursor; idx >= 0 && idx < len(m.accounts) {
				acct := m.accounts[idx]
				m.renaming = true
				m.renameTargetID = acct.ID
				m.renameInput = textinput.New()
				m.renameInput.SetValue(acct.Name)
				m.renameInput.CharLimit = 60
				m.renameInput.Focus()
				m.err = nil
			}
		}
	}
	return m, nil
}

func (m *accountListModel) selectedID() string {
	if m.cursor >= 0 && m.cursor < len(m.accounts) {
		return m.accounts[m.cursor].ID
	}
	return ""
}

func (m *accountListModel) view() string {
	if m.loading {
		return "Loading accounts..."
	}
	if m.err != nil {
		return errorStyle.Render("Error: " + m.err.Error())
	}
	if len(m.accounts) == 0 {
		return dimStyle.Render("No accounts found. Press 'n' to create one.")
	}

	var b strings.Builder

	b.WriteString(titleStyle.Render("Accounts"))
	b.WriteString("\n")

	// Header
	header := fmt.Sprintf("  %-12s %-28s %6s %-15s %18s %s", "ID", "NAME", "CODE", "CATEGORY", "BALANCE", "CCY")
	b.WriteString(headerStyle.Render(header))
	b.WriteString("\n")

	// Rows
	maxRows := m.height - 4
	if maxRows < 1 {
		maxRows = 10
	}

	start := 0
	if m.cursor >= maxRows {
		start = m.cursor - maxRows + 1
	}

	for i := start; i < len(m.accounts) && i < start+maxRows; i++ {
		a := m.accounts[i]
		name := a.Name
		if len(name) > 26 {
			name = name[:26] + ".."
		}

		balStr := ""
		if bal, ok := m.balances[a.ID]; ok {
			balStr = ledger.FormatAmount(bal.Balance, bal.Currency)
		}
		line := fmt.Sprintf("  %-12s %-28s %6d %-15s %18s %s", a.ID, name, a.Code, a.Category, balStr, a.Currency)
		if i == m.cursor {
			b.WriteString(selectedStyle.Render("> " + line[2:]))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}

	if m.renaming {
		b.WriteString(fmt.Sprintf("\n  Rename %s: ", m.renameTargetID))
		b.WriteString(m.renameInput.View())
	} else if m.confirmDelete {
		b.WriteString("\n" + errorStyle.Render(fmt.Sprintf("  Delete account %q? (y/n)", m.deleteTargetID)))
	} else {
		b.WriteString(fmt.Sprintf("\n  %d accounts", len(m.accounts)))
	}

	// IFRS callout for selected account
	if idx := m.cursor; idx >= 0 && idx < len(m.accounts) {
		acct := m.accounts[idx]
		if entry := ledger.LookupChartEntry(acct.Code); entry != nil {
			info := fmt.Sprintf(
				"%s  IFRS %d — %s\n%s  %s",
				headerStyle.Render(""), entry.Code, entry.Name,
				dimStyle.Render(""), entry.Description,
			)
			b.WriteString("\n\n")
			b.WriteString(boxStyle.Render(info))
		}
	}

	return b.String()
}
