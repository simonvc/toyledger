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
	w := m.width
	if w < 60 {
		w = 80
	}

	// Flexible NAME column: total fixed = indent(4)+acctID(6)+gaps+native(12)+ccy(3)+gel(12)+" GEL"(4) = 46
	nameW := w - 46
	if nameW < 10 {
		nameW = 10
	}
	if nameW > 40 {
		nameW = 40
	}
	// Width of the label field in total rows so GEL amount aligns with line items
	totalLabelW := nameW + 25

	b.WriteString(titleStyle.Render(centerStr("BALANCE SHEET", w)))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(centerStr("Reporting currency: "+ledger.ReportingCurrency, w)))
	b.WriteString("\n\n")

	renderSection := func(title string, lines []ledger.BalanceSheetLine) int64 {
		b.WriteString(fmt.Sprintf("  %s\n", headerStyle.Render(title)))
		if len(lines) == 0 {
			b.WriteString(dimStyle.Render("    (no entries)") + "\n\n")
			return 0
		}
		var totalGEL int64
		for _, l := range lines {
			name := l.AccountName
			if len(name) > nameW-2 {
				name = name[:nameW-2] + ".."
			}
			nativeAmt := formatBalanceSheetAmt(l.Balance, l.Currency)
			gelAmt := ledger.ToGEL(l.Balance, l.Currency)
			totalGEL += gelAmt
			gelStr := formatBalanceSheetAmt(gelAmt, ledger.ReportingCurrency)
			b.WriteString(fmt.Sprintf("    %-6s %-*s %12s %-3s  %12s GEL\n",
				l.AccountID, nameW, name, nativeAmt, l.Currency, gelStr))
		}
		b.WriteString(fmt.Sprintf("    %s\n", strings.Repeat("─", w-8)))
		b.WriteString(fmt.Sprintf("    %-*s %12s GEL\n",
			totalLabelW, "Total "+title, formatBalanceSheetAmt(totalGEL, ledger.ReportingCurrency)))
		b.WriteString("\n")
		return totalGEL
	}

	renderSection("Assets", m.bs.Assets)
	totalLiabGEL := renderSection("Liabilities", m.bs.Liabilities)
	totalEquityGEL := renderSection("Equity", m.bs.Equity)

	b.WriteString(fmt.Sprintf("    %s\n", strings.Repeat("═", w-8)))
	b.WriteString(fmt.Sprintf("    %-*s %12s GEL\n",
		totalLabelW, "Total L + E", formatBalanceSheetAmt(totalLiabGEL+totalEquityGEL, ledger.ReportingCurrency)))

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
