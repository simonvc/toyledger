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

	// Compute column widths from actual data (longest value + 1).
	acctW := len("ID")
	nameW := len("Name")
	nativeW := 1
	ccyW := 3
	gelW := 1
	allLines := make([]ledger.BalanceSheetLine, 0, len(m.bs.Assets)+len(m.bs.Liabilities)+len(m.bs.Equity))
	allLines = append(allLines, m.bs.Assets...)
	allLines = append(allLines, m.bs.Liabilities...)
	allLines = append(allLines, m.bs.Equity...)
	for _, l := range allLines {
		if l2 := len(l.AccountID); l2 > acctW {
			acctW = l2
		}
		if l2 := len(l.AccountName); l2 > nameW {
			nameW = l2
		}
		if l2 := len(formatBalanceSheetAmt(l.Balance, l.Currency)); l2 > nativeW {
			nativeW = l2
		}
		if l2 := len(l.Currency); l2 > ccyW {
			ccyW = l2
		}
		gelAmt := ledger.ToGEL(l.Balance, l.Currency)
		if l2 := len(formatBalanceSheetAmt(gelAmt, ledger.ReportingCurrency)); l2 > gelW {
			gelW = l2
		}
	}
	// +1 for inter-column gap
	acctW++
	ccyW++
	// Cap name to remaining terminal width
	fixedW := 4 + acctW + nativeW + ccyW + gelW + 4 // indent(4) + cols + " GEL"(4)
	maxNameW := w - fixedW
	if maxNameW < 10 {
		maxNameW = 10
	}
	if nameW > maxNameW {
		nameW = maxNameW
	}
	if nameW > 40 {
		nameW = 40
	}
	nameW++ // +1 gap after name
	// Width of the label field in total rows so GEL amount aligns with line items
	totalLabelW := acctW + nameW + nativeW + ccyW

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
			b.WriteString(fmt.Sprintf("    %-*s%-*s%*s %-*s%*s GEL\n",
				acctW, l.AccountID, nameW, name, nativeW, nativeAmt, ccyW, l.Currency, gelW, gelStr))
		}
		b.WriteString(fmt.Sprintf("    %s\n", strings.Repeat("─", totalLabelW+gelW+4)))
		b.WriteString(fmt.Sprintf("    %-*s%*s GEL\n",
			totalLabelW, "Total "+title, gelW, formatBalanceSheetAmt(totalGEL, ledger.ReportingCurrency)))
		b.WriteString("\n")
		return totalGEL
	}

	renderSection("Assets", m.bs.Assets)
	totalLiabGEL := renderSection("Liabilities", m.bs.Liabilities)
	totalEquityGEL := renderSection("Equity", m.bs.Equity)

	b.WriteString(fmt.Sprintf("    %s\n", strings.Repeat("═", totalLabelW+gelW+4)))
	b.WriteString(fmt.Sprintf("    %-*s%*s GEL\n",
		totalLabelW, "Total L + E", gelW, formatBalanceSheetAmt(totalLiabGEL+totalEquityGEL, ledger.ReportingCurrency)))

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
