package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/simonvc/miniledger/internal/client"
	"github.com/simonvc/miniledger/internal/ledger"
)

type accountsLoadedMsg struct {
	accounts []ledger.Account
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

type accountListModel struct {
	accounts       []ledger.Account
	cursor         int
	loading        bool
	err            error
	width          int
	height         int
	confirmDelete  bool
	deleteTargetID string
}

func (m *accountListModel) init(c *client.Client) tea.Cmd {
	m.loading = true
	return func() tea.Msg {
		accounts, err := c.ListAccounts(context.Background(), "", nil)
		return accountsLoadedMsg{accounts: accounts, err: err}
	}
}

func (m accountListModel) update(msg tea.Msg) (accountListModel, tea.Cmd) {
	switch msg := msg.(type) {
	case accountsLoadedMsg:
		m.loading = false
		m.accounts = msg.accounts
		m.err = msg.err

	case accountDeletedMsg:
		m.confirmDelete = false
		m.deleteTargetID = ""
		if msg.err != nil {
			m.err = msg.err
		}

	case tea.KeyMsg:
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
	header := fmt.Sprintf("  %-12s %-30s %6s %-15s %-7s %s", "ID", "NAME", "CODE", "CATEGORY", "NORMAL", "CCY")
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
		if len(name) > 28 {
			name = name[:28] + ".."
		}

		normal := ledger.NormalBalance(a.Category)
		line := fmt.Sprintf("  %-12s %-30s %6d %-15s %-7s %s", a.ID, name, a.Code, a.Category, normal, a.Currency)
		if i == m.cursor {
			b.WriteString(selectedStyle.Render("> " + line[2:]))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}

	if m.confirmDelete {
		b.WriteString("\n" + errorStyle.Render(fmt.Sprintf("  Delete account %q? (y/n)", m.deleteTargetID)))
	} else {
		b.WriteString(fmt.Sprintf("\n  %d accounts", len(m.accounts)))
	}

	return b.String()
}
