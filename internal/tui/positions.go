package tui

import (
	"context"
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/simonvc/miniledger/internal/client"
	"github.com/simonvc/miniledger/internal/ledger"
)

type positionsLoadedMsg struct {
	accounts []ledger.Account
	balances map[string]*client.BalanceResponse
	err      error
}

type currencyPosition struct {
	Currency    string
	Assets      int64 // positive, minor units in native currency
	Liabilities int64 // positive (absolute), minor units
	Equity      int64 // positive (absolute), minor units
	Net         int64 // raw sum of all balances in this currency
	GELEquiv    int64 // ToGEL(Net, Currency)
}

type positionsModel struct {
	positions []currencyPosition
	totalGEL  int64
	loading   bool
	err       error
	width     int
	height    int
}

func (m *positionsModel) init(c *client.Client) tea.Cmd {
	m.loading = true
	return func() tea.Msg {
		accounts, err := c.ListAccounts(context.Background(), "", nil)
		if err != nil {
			return positionsLoadedMsg{err: err}
		}
		balances := make(map[string]*client.BalanceResponse, len(accounts))
		for _, a := range accounts {
			if a.Currency == "*" {
				continue // skip wildcard accounts like ~fx
			}
			bal, err := c.GetAccountBalance(context.Background(), a.ID)
			if err == nil {
				balances[a.ID] = bal
			}
		}
		return positionsLoadedMsg{accounts: accounts, balances: balances}
	}
}

func (m positionsModel) update(msg tea.Msg) (positionsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case positionsLoadedMsg:
		m.loading = false
		m.err = msg.err
		if msg.err == nil {
			m.compute(msg.accounts, msg.balances)
		}
	}
	return m, nil
}

func (m *positionsModel) compute(accounts []ledger.Account, balances map[string]*client.BalanceResponse) {
	type bucket struct {
		assets      int64
		liabilities int64
		equity      int64
		net         int64
	}
	byCurrency := make(map[string]*bucket)

	ensureBucket := func(ccy string) *bucket {
		if b, ok := byCurrency[ccy]; ok {
			return b
		}
		b := &bucket{}
		byCurrency[ccy] = b
		return b
	}

	for _, a := range accounts {
		// Skip wildcard-currency accounts (~fx) — they are booking
		// intermediaries, not real holdings. The bank's actual open
		// currency exposure is visible from real accounts only.
		if a.Currency == "*" {
			continue
		}
		bal, ok := balances[a.ID]
		if !ok {
			continue
		}
		b := ensureBucket(a.Currency)
		b.net += bal.Balance

		switch a.Category {
		case ledger.CategoryAssets:
			b.assets += bal.Balance
		case ledger.CategoryLiabilities:
			// Liabilities are credit-normal (negative balance = positive liability)
			b.liabilities += -bal.Balance
		case ledger.CategoryEquity:
			b.equity += -bal.Balance
		}
	}

	// Build sorted positions
	currencies := make([]string, 0, len(byCurrency))
	for ccy := range byCurrency {
		currencies = append(currencies, ccy)
	}
	sort.Strings(currencies)

	m.positions = make([]currencyPosition, 0, len(currencies))
	m.totalGEL = 0
	for _, ccy := range currencies {
		b := byCurrency[ccy]
		gelEquiv := ledger.ToGEL(b.net, ccy)
		m.totalGEL += gelEquiv
		m.positions = append(m.positions, currencyPosition{
			Currency:    ccy,
			Assets:      b.assets,
			Liabilities: b.liabilities,
			Equity:      b.equity,
			Net:         b.net,
			GELEquiv:    gelEquiv,
		})
	}
}

func (m *positionsModel) view() string {
	if m.loading {
		return "Loading positions..."
	}
	if m.err != nil {
		return errorStyle.Render("Error: " + m.err.Error())
	}
	if len(m.positions) == 0 {
		return dimStyle.Render("No currency positions found.")
	}

	var b strings.Builder

	b.WriteString(titleStyle.Render("Open Currency Positions"))
	b.WriteString("\n\n")

	// Column widths
	ccyW := 5
	assetsW := len("ASSETS (Long)")
	liabW := len("LIABS (Short)")
	eqW := len("EQUITY")
	netW := len("NET")
	gelW := len("GEL EQUIV")

	for _, p := range m.positions {
		if l := len(ledger.FormatAmount(p.Assets, p.Currency)); l > assetsW {
			assetsW = l
		}
		if l := len(ledger.FormatAmount(p.Liabilities, p.Currency)); l > liabW {
			liabW = l
		}
		if l := len(ledger.FormatAmount(p.Equity, p.Currency)); l > eqW {
			eqW = l
		}
		if l := len(ledger.FormatAmount(p.Net, p.Currency)); l > netW {
			netW = l
		}
		if l := len(ledger.FormatAmount(p.GELEquiv, ledger.ReportingCurrency)); l > gelW {
			gelW = l
		}
	}
	// Padding
	assetsW += 2
	liabW += 2
	eqW += 2
	netW += 2
	gelW += 2

	header := fmt.Sprintf("  %-*s%*s%*s%*s%*s%*s",
		ccyW, "CCY",
		assetsW, "ASSETS (Long)",
		liabW, "LIABS (Short)",
		eqW, "EQUITY",
		netW, "NET",
		gelW, "GEL EQUIV")
	b.WriteString(headerStyle.Render(header))
	b.WriteString("\n")

	for _, p := range m.positions {
		assets := ledger.FormatAmount(p.Assets, p.Currency)
		liab := ledger.FormatAmount(p.Liabilities, p.Currency)
		eq := ledger.FormatAmount(p.Equity, p.Currency)
		net := ledger.FormatAmount(p.Net, p.Currency)
		gel := ledger.FormatAmount(p.GELEquiv, ledger.ReportingCurrency)

		line := fmt.Sprintf("  %-*s%*s%*s%*s%*s%*s",
			ccyW, p.Currency,
			assetsW, assets,
			liabW, liab,
			eqW, eq,
			netW, net,
			gelW, gel)

		if p.Net > 0 {
			b.WriteString(debitStyle.Render(line))
		} else if p.Net < 0 {
			b.WriteString(creditStyle.Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}

	// Total line
	totalW := ccyW + assetsW + liabW + eqW + netW
	totalLabel := "Total Open Position"
	totalGEL := ledger.FormatAmount(m.totalGEL, ledger.ReportingCurrency) + " GEL"

	b.WriteString(fmt.Sprintf("  %s\n", strings.Repeat("─", totalW+gelW)))
	if m.totalGEL > 0 {
		b.WriteString(debitStyle.Render(fmt.Sprintf("  %-*s%*s", totalW-2, totalLabel, gelW, totalGEL)))
	} else if m.totalGEL < 0 {
		b.WriteString(creditStyle.Render(fmt.Sprintf("  %-*s%*s", totalW-2, totalLabel, gelW, totalGEL)))
	} else {
		b.WriteString(fmt.Sprintf("  %-*s%*s", totalW-2, totalLabel, gelW, totalGEL))
	}
	b.WriteString("\n\n")

	b.WriteString(dimStyle.Render("  Excludes ~fx booking intermediary — shows real account exposure only.") + "\n")
	b.WriteString(dimStyle.Render("  Long = debit-normal (assets), Short = credit-normal (liabilities).") + "\n")

	return b.String()
}
