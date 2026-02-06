package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/simonvc/miniledger/internal/client"
	"github.com/simonvc/miniledger/internal/ledger"
)

type balanceSheetLoadedMsg struct {
	bs  *ledger.BalanceSheet
	err error
}

type balanceSheetModel struct {
	bs      *ledger.BalanceSheet
	loading bool
	err     error
	width   int
	height  int
}

func (m *balanceSheetModel) init(c *client.Client) tea.Cmd {
	m.loading = true
	return func() tea.Msg {
		bs, err := c.BalanceSheet(context.Background())
		return balanceSheetLoadedMsg{bs: bs, err: err}
	}
}

func (m balanceSheetModel) update(msg tea.Msg) (balanceSheetModel, tea.Cmd) {
	switch msg := msg.(type) {
	case balanceSheetLoadedMsg:
		m.loading = false
		m.bs = msg.bs
		m.err = msg.err
	}
	return m, nil
}

func (m *balanceSheetModel) view() string {
	if m.loading {
		return "Loading balance sheet..."
	}
	if m.err != nil {
		return errorStyle.Render("Error: " + m.err.Error())
	}
	if m.bs == nil {
		return dimStyle.Render("No data available.")
	}

	var b strings.Builder
	w := 60

	b.WriteString(titleStyle.Render(centerStr("BALANCE SHEET", w)))
	b.WriteString("\n\n")

	renderSection := func(title string, lines []ledger.BalanceSheetLine, total int64) {
		b.WriteString(fmt.Sprintf("  %s\n", headerStyle.Render(title)))
		if len(lines) == 0 {
			b.WriteString(dimStyle.Render("    (no entries)") + "\n")
		}
		for _, l := range lines {
			name := l.AccountName
			if len(name) > 28 {
				name = name[:28] + ".."
			}
			amt := formatBalanceSheetAmt(l.Balance, l.Currency)
			b.WriteString(fmt.Sprintf("    %-6s %-28s %16s\n", l.AccountID, name, amt))
		}
		b.WriteString(fmt.Sprintf("    %s\n", strings.Repeat("─", w-8)))
		b.WriteString(fmt.Sprintf("    %-35s %16s\n", "Total "+title, formatBalanceSheetAmt(total, "USD")))
		b.WriteString("\n")
	}

	renderSection("Assets", m.bs.Assets, m.bs.TotalAssets)
	renderSection("Liabilities", m.bs.Liabilities, m.bs.TotalLiabilities)
	renderSection("Equity", m.bs.Equity, m.bs.TotalEquity)

	b.WriteString(fmt.Sprintf("    %s\n", strings.Repeat("═", w-8)))
	b.WriteString(fmt.Sprintf("    %-35s %16s\n", "Total L + E",
		formatBalanceSheetAmt(m.bs.TotalLiabilities+m.bs.TotalEquity, "USD")))

	b.WriteString("\n")
	if m.bs.Balanced {
		b.WriteString(successStyle.Render("    [BALANCED]"))
	} else {
		b.WriteString(errorStyle.Render("    [UNBALANCED!]"))
	}

	return b.String()
}

func formatBalanceSheetAmt(amount int64, currency string) string {
	if amount < 0 {
		return "(" + ledger.FormatAmount(-amount, currency) + ")"
	}
	return ledger.FormatAmount(amount, currency)
}

func centerStr(s string, w int) string {
	if len(s) >= w {
		return s
	}
	pad := (w - len(s)) / 2
	return strings.Repeat(" ", pad) + s
}
