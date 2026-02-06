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

type txnsLoadedMsg struct {
	txns []ledger.Transaction
	err  error
}

type txnListModel struct {
	txns    []ledger.Transaction
	cursor  int
	loading bool
	err     error
	width   int
	height  int
}

func (m *txnListModel) init(c *client.Client) tea.Cmd {
	m.loading = true
	return func() tea.Msg {
		txns, err := c.ListTransactions(context.Background(), "")
		return txnsLoadedMsg{txns: txns, err: err}
	}
}

func (m txnListModel) update(msg tea.Msg) (txnListModel, tea.Cmd) {
	switch msg := msg.(type) {
	case txnsLoadedMsg:
		m.loading = false
		m.txns = msg.txns
		m.err = msg.err

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, keys.Down):
			if m.cursor < len(m.txns)-1 {
				m.cursor++
			}
		}
	}
	return m, nil
}

func (m *txnListModel) selectedID() string {
	if m.cursor >= 0 && m.cursor < len(m.txns) {
		return m.txns[m.cursor].ID
	}
	return ""
}

func (m *txnListModel) view() string {
	if m.loading {
		return "Loading transactions..."
	}
	if m.err != nil {
		return errorStyle.Render("Error: " + m.err.Error())
	}
	if len(m.txns) == 0 {
		return dimStyle.Render("No transactions found.")
	}

	var b strings.Builder

	b.WriteString(titleStyle.Render("Transactions"))
	b.WriteString("\n")

	header := fmt.Sprintf("  %-36s %-20s %-8s %s", "ID", "DATE", "ENTRIES", "DESCRIPTION")
	b.WriteString(headerStyle.Render(header))
	b.WriteString("\n")

	maxRows := m.height - 4
	if maxRows < 1 {
		maxRows = 10
	}

	start := 0
	if m.cursor >= maxRows {
		start = m.cursor - maxRows + 1
	}

	for i := start; i < len(m.txns) && i < start+maxRows; i++ {
		t := m.txns[i]
		desc := t.Description
		if len(desc) > 30 {
			desc = desc[:28] + ".."
		}
		idShort := t.ID
		if len(idShort) > 34 {
			idShort = idShort[:34] + ".."
		}

		line := fmt.Sprintf("  %-36s %-20s %-8d %s",
			idShort,
			t.PostedAt.Format("2006-01-02 15:04"),
			len(t.Entries),
			desc,
		)
		if i == m.cursor {
			b.WriteString(selectedStyle.Render("> " + line[2:]))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}

	b.WriteString(fmt.Sprintf("\n  %d transactions", len(m.txns)))
	return b.String()
}
