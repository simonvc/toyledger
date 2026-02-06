package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/simonvc/miniledger/internal/client"
	"github.com/simonvc/miniledger/internal/ledger"
)

type ratiosLoadedMsg struct {
	ratios *ledger.RegulatoryRatios
	err    error
}

type ratiosModel struct {
	ratios  *ledger.RegulatoryRatios
	loading bool
	err     error
	width   int
	height  int
}

func (m *ratiosModel) init(c *client.Client) tea.Cmd {
	m.loading = true
	return func() tea.Msg {
		ratios, err := c.RegulatoryRatios(context.Background())
		return ratiosLoadedMsg{ratios: ratios, err: err}
	}
}

func (m ratiosModel) update(msg tea.Msg) (ratiosModel, tea.Cmd) {
	switch msg := msg.(type) {
	case ratiosLoadedMsg:
		m.loading = false
		m.ratios = msg.ratios
		m.err = msg.err
	}
	return m, nil
}

var (
	ratioGreen  = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	ratioYellow = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	ratioRed    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
)

func ratioStyle(value float64, warnThreshold, dangerThreshold float64) lipgloss.Style {
	if value < dangerThreshold {
		return ratioRed
	}
	if value < warnThreshold {
		return ratioYellow
	}
	return ratioGreen
}

func ratioBar(value float64, maxPct float64, width int) string {
	if width < 10 {
		width = 40
	}
	barWidth := width - 10
	if barWidth > 50 {
		barWidth = 50
	}
	filled := int(value / maxPct * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}
	if filled < 0 {
		filled = 0
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
}

func (m *ratiosModel) view() string {
	if m.loading {
		return "Loading ratios..."
	}
	if m.err != nil {
		return errorStyle.Render("Error: " + m.err.Error())
	}
	if m.ratios == nil {
		return dimStyle.Render("No data available.")
	}

	r := m.ratios
	var b strings.Builder

	b.WriteString(titleStyle.Render("Regulatory Ratios"))
	b.WriteString("\n\n")

	renderRatio := func(name string, value float64, warn, danger, maxPct float64, num, denom int64, numLabel, denomLabel, currency string) {
		style := ratioStyle(value, warn, danger)
		status := "OK"
		if value < danger {
			status = "DANGER"
		} else if value < warn {
			status = "WARNING"
		}

		b.WriteString(fmt.Sprintf("  %s\n", headerStyle.Render(name)))
		bar := ratioBar(value, maxPct, m.width-8)
		b.WriteString(fmt.Sprintf("  %s %s\n", style.Render(bar), style.Render(fmt.Sprintf("%6.1f%%  [%s]", value, status))))
		b.WriteString(dimStyle.Render(fmt.Sprintf("    %s: %s %s  /  %s: %s %s",
			numLabel, ledger.FormatAmount(num, currency), currency,
			denomLabel, ledger.FormatAmount(denom, currency), currency)))
		b.WriteString("\n\n")
	}

	renderRatio("Capital Adequacy Ratio (CAR)",
		r.CapitalAdequacy, 12, 8, 50,
		r.Equity, r.TotalAssets, "Equity", "Total Assets", "USD")

	renderRatio("Leverage Ratio",
		r.LeverageRatio, 5, 3, 50,
		r.Equity, r.TotalAssets, "Equity", "Total Assets", "USD")

	renderRatio("Reserve Ratio",
		r.ReserveRatio, 20, 10, 100,
		r.Reserves, r.CustomerDeposits, "Reserves (1060)", "Customer Deposits (2020)", "USD")

	// Summary
	b.WriteString(fmt.Sprintf("  %s\n", strings.Repeat("─", 52)))
	allGood := r.CapitalAdequacy >= 8 && r.LeverageRatio >= 3 && (r.CustomerDeposits == 0 || r.ReserveRatio >= 10)
	if allGood {
		b.WriteString(successStyle.Render("  All ratios within regulatory limits"))
	} else {
		b.WriteString(errorStyle.Render("  One or more ratios below regulatory minimums"))
	}

	return b.String()
}

// RenderRatioImpact returns a string showing current → projected ratios for use in confirm views.
func RenderRatioImpact(current, projected *ledger.RegulatoryRatios) string {
	if current == nil {
		return ""
	}
	if projected == nil {
		projected = current
	}

	var b strings.Builder
	b.WriteString("  Ratio Impact:\n")

	renderLine := func(name string, cur, proj float64, warn, danger float64) {
		style := ratioStyle(proj, warn, danger)
		arrow := dimStyle.Render("→")
		curStr := fmt.Sprintf("%5.1f%%", cur)
		projStr := style.Render(fmt.Sprintf("%5.1f%%", proj))
		b.WriteString(fmt.Sprintf("    %-10s %s %s %s\n", name, curStr, arrow, projStr))
	}

	renderLine("CAR:", current.CapitalAdequacy, projected.CapitalAdequacy, 12, 8)
	renderLine("Leverage:", current.LeverageRatio, projected.LeverageRatio, 5, 3)
	if current.CustomerDeposits > 0 || projected.CustomerDeposits > 0 {
		renderLine("Reserve:", current.ReserveRatio, projected.ReserveRatio, 20, 10)
	}

	return b.String()
}
