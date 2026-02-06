package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/simonvc/miniledger/internal/client"
	"github.com/simonvc/miniledger/internal/ledger"
)

type accountDetailLoadedMsg struct {
	account *ledger.Account
	balance *client.BalanceResponse
	entries []ledger.Entry
	err     error
}

type accountDetailModel struct {
	account *ledger.Account
	balance *client.BalanceResponse
	entries []ledger.Entry
	loading bool
	err     error
	width   int
}

func (m *accountDetailModel) init(c *client.Client, id string) tea.Cmd {
	m.loading = true
	return func() tea.Msg {
		acct, err := c.GetAccount(context.Background(), id)
		if err != nil {
			return accountDetailLoadedMsg{err: err}
		}
		bal, err := c.GetAccountBalance(context.Background(), id)
		if err != nil {
			return accountDetailLoadedMsg{account: acct, err: err}
		}
		entries, err := c.ListAccountEntries(context.Background(), id)
		return accountDetailLoadedMsg{account: acct, balance: bal, entries: entries, err: err}
	}
}

func (m accountDetailModel) update(msg tea.Msg) (accountDetailModel, tea.Cmd) {
	switch msg := msg.(type) {
	case accountDetailLoadedMsg:
		m.loading = false
		m.account = msg.account
		m.balance = msg.balance
		m.entries = msg.entries
		m.err = msg.err
	}
	return m, nil
}

func (m *accountDetailModel) view() string {
	if m.loading {
		return "Loading account..."
	}
	if m.err != nil {
		return errorStyle.Render("Error: " + m.err.Error())
	}
	if m.account == nil {
		return ""
	}

	var b strings.Builder

	b.WriteString(titleStyle.Render(fmt.Sprintf("Account: %s", m.account.ID)))
	b.WriteString("\n")

	b.WriteString(fmt.Sprintf("%s %s\n", labelStyle.Render("Name:"), m.account.Name))
	b.WriteString(fmt.Sprintf("%s %d\n", labelStyle.Render("Code:"), m.account.Code))
	b.WriteString(fmt.Sprintf("%s %s\n", labelStyle.Render("Category:"), ledger.CategoryLabel(m.account.Category)))
	b.WriteString(fmt.Sprintf("%s %s\n", labelStyle.Render("Currency:"), m.account.Currency))
	if m.balance != nil {
		b.WriteString(fmt.Sprintf("%s %s %s\n", labelStyle.Render("Balance:"), m.balance.Formatted, m.balance.Currency))
	}
	b.WriteString(fmt.Sprintf("%s %v\n", labelStyle.Render("System:"), m.account.IsSystem))
	b.WriteString("\n")

	if len(m.entries) == 0 {
		b.WriteString(dimStyle.Render("  No entries."))
	} else {
		header := fmt.Sprintf("  %-4s %-36s %15s %s", "TYPE", "TRANSACTION", "AMOUNT", "CCY")
		b.WriteString(headerStyle.Render(header))
		b.WriteString("\n")

		for _, e := range m.entries {
			direction := "DR"
			amt := e.Amount
			if amt < 0 {
				direction = "CR"
				amt = -amt
			}
			formatted := ledger.FormatAmount(amt, e.Currency)
			txnShort := e.TransactionID
			if len(txnShort) > 34 {
				txnShort = txnShort[:34] + ".."
			}
			line := fmt.Sprintf("  %-4s %-36s %15s %s", direction, txnShort, formatted, e.Currency)
			if direction == "DR" {
				b.WriteString(debitStyle.Render(line))
			} else {
				b.WriteString(creditStyle.Render(line))
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("\n" + dimStyle.Render("  Press ESC to go back"))
	return b.String()
}
