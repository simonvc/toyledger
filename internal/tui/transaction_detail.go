package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/simonvc/miniledger/internal/client"
	"github.com/simonvc/miniledger/internal/ledger"
)

type txnDetailLoadedMsg struct {
	txn *ledger.Transaction
	err error
}

type txnDetailModel struct {
	txn     *ledger.Transaction
	loading bool
	err     error
	width   int
}

func (m *txnDetailModel) init(c *client.Client, id string) tea.Cmd {
	m.loading = true
	return func() tea.Msg {
		txn, err := c.GetTransaction(context.Background(), id)
		return txnDetailLoadedMsg{txn: txn, err: err}
	}
}

func (m txnDetailModel) update(msg tea.Msg) (txnDetailModel, tea.Cmd) {
	switch msg := msg.(type) {
	case txnDetailLoadedMsg:
		m.loading = false
		m.txn = msg.txn
		m.err = msg.err
	}
	return m, nil
}

func (m *txnDetailModel) view() string {
	if m.loading {
		return "Loading transaction..."
	}
	if m.err != nil {
		return errorStyle.Render("Error: " + m.err.Error())
	}
	if m.txn == nil {
		return ""
	}

	var b strings.Builder

	b.WriteString(titleStyle.Render(fmt.Sprintf("Transaction: %s", m.txn.ID)))
	b.WriteString("\n")

	b.WriteString(fmt.Sprintf("%s %s\n", labelStyle.Render("Description:"), m.txn.Description))
	b.WriteString(fmt.Sprintf("%s %s\n", labelStyle.Render("Posted:"), m.txn.PostedAt.Format("2006-01-02 15:04:05")))
	b.WriteString(fmt.Sprintf("%s %v\n", labelStyle.Render("Finalized:"), m.txn.Finalized))
	b.WriteString("\n")

	header := fmt.Sprintf("  %-4s %-14s %15s %15s %s", "TYPE", "ACCOUNT", "DEBIT", "CREDIT", "CCY")
	b.WriteString(headerStyle.Render(header))
	b.WriteString("\n")

	for _, e := range m.txn.Entries {
		debit := ""
		credit := ""
		if e.Amount >= 0 {
			debit = ledger.FormatAmount(e.Amount, e.Currency)
		} else {
			credit = ledger.FormatAmount(-e.Amount, e.Currency)
		}

		direction := "DR"
		if e.Amount < 0 {
			direction = "CR"
		}

		line := fmt.Sprintf("  %-4s %-14s %15s %15s %s", direction, e.AccountID, debit, credit, e.Currency)
		if e.Amount >= 0 {
			b.WriteString(debitStyle.Render(line))
		} else {
			b.WriteString(creditStyle.Render(line))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n" + dimStyle.Render("  Press ESC to go back"))
	return b.String()
}
