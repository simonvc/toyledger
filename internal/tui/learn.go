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

type learnState int

const (
	learnBrowse learnState = iota
	learnAccounts
	learnAmount
	learnCurrency
	learnAmount2   // destination amount (multi-currency)
	learnCurrency2 // destination currency (multi-currency)
	learnConfirm
)

type learnTxnCreatedMsg struct {
	txn *ledger.Transaction
	err error
}

type learnAccountsLoadedMsg struct {
	accounts []ledger.Account
	err      error
}

type learnRatiosLoadedMsg struct {
	ratios *ledger.RegulatoryRatios
	err    error
}

type learnModel struct {
	state       learnState
	cursor      int
	templates   []ledger.Template
	amountInput textinput.Model
	currencyIdx int
	curOptions  []string
	err         error
	statusMsg   string
	width       int
	height      int

	// Destination amount/currency for multi-currency templates
	amountInput2 textinput.Model
	currencyIdx2 int

	// Account editing state
	acctInputs []textinput.Model
	acctIdx    int
	accounts   []ledger.Account

	// Ratios for impact preview
	ratios *ledger.RegulatoryRatios
}

func (m *learnModel) init() {
	m.templates = ledger.Templates
	m.curOptions = ledger.CurrencyCodes()
	for i, c := range m.curOptions {
		if c == "USD" {
			m.currencyIdx = i
			m.currencyIdx2 = i
			break
		}
	}
}

func (m *learnModel) isMultiCurrency() bool {
	if tmpl := m.selected(); tmpl != nil {
		return tmpl.IsMultiCurrency()
	}
	return false
}

func (m learnModel) update(msg tea.Msg, c *client.Client) (learnModel, tea.Cmd) {
	switch msg := msg.(type) {
	case learnTxnCreatedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.state = learnConfirm
			return m, nil
		}
		m.statusMsg = fmt.Sprintf("Transaction %s created!", msg.txn.ID[:8])
		m.state = learnBrowse
		m.err = nil
		return m, nil

	case learnAccountsLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.accounts = msg.accounts
		return m, nil

	case learnRatiosLoadedMsg:
		if msg.err == nil {
			m.ratios = msg.ratios
		}
		return m, nil

	case tea.KeyMsg:
		prevState := m.state
		var cmd tea.Cmd
		switch m.state {
		case learnBrowse:
			m, cmd = m.updateBrowse(msg, c)
		case learnAccounts:
			m, cmd = m.updateAccounts(msg)
		case learnAmount:
			m, cmd = m.updateAmount(msg)
		case learnCurrency:
			m, cmd = m.updateCurrency(msg)
		case learnAmount2:
			m, cmd = m.updateAmount2(msg)
		case learnCurrency2:
			m, cmd = m.updateCurrency2(msg)
		case learnConfirm:
			m, cmd = m.updateConfirm(msg, c)
		}

		if prevState != learnConfirm && m.state == learnConfirm {
			loadRatios := func() tea.Msg {
				ratios, err := c.RegulatoryRatios(context.Background())
				return learnRatiosLoadedMsg{ratios: ratios, err: err}
			}
			if cmd != nil {
				return m, tea.Batch(cmd, loadRatios)
			}
			return m, loadRatios
		}
		return m, cmd
	}
	return m, nil
}

func (m learnModel) updateBrowse(msg tea.KeyMsg, c *client.Client) (learnModel, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Up):
		if m.cursor > 0 {
			m.cursor--
		}
	case key.Matches(msg, keys.Down):
		if m.cursor < len(m.templates)-1 {
			m.cursor++
		}
	case key.Matches(msg, keys.Enter):
		tmpl := m.templates[m.cursor]
		m.acctInputs = make([]textinput.Model, len(tmpl.Entries))
		for i, e := range tmpl.Entries {
			ti := textinput.New()
			ti.Placeholder = fmt.Sprintf("account ID (CoA %d)", e.CoACode)
			ti.CharLimit = 40
			ti.SetValue(ledger.DefaultAccountForCoA(e.CoACode))
			m.acctInputs[i] = ti
		}
		m.acctIdx = 0
		m.acctInputs[0].Focus()
		m.state = learnAccounts
		m.err = nil
		m.statusMsg = ""
		return m, func() tea.Msg {
			accounts, err := c.ListAccounts(context.Background(), "", nil)
			return learnAccountsLoadedMsg{accounts: accounts, err: err}
		}
	}
	return m, nil
}

func (m learnModel) updateAccounts(msg tea.KeyMsg) (learnModel, tea.Cmd) {
	if key.Matches(msg, keys.Escape) {
		m.state = learnBrowse
		m.err = nil
		return m, nil
	}
	if key.Matches(msg, keys.Enter) {
		val := strings.TrimSpace(m.acctInputs[m.acctIdx].Value())
		if val == "" {
			m.err = fmt.Errorf("account ID is required")
			return m, nil
		}
		m.err = nil
		m.acctInputs[m.acctIdx].Blur()

		if m.acctIdx < len(m.acctInputs)-1 {
			m.acctIdx++
			m.acctInputs[m.acctIdx].Focus()
			return m, nil
		}
		m.state = learnAmount
		m.amountInput = textinput.New()
		if m.isMultiCurrency() {
			m.amountInput.Placeholder = "source amount, e.g. 1000.00"
		} else {
			m.amountInput.Placeholder = "e.g. 1000.00"
		}
		m.amountInput.CharLimit = 20
		m.amountInput.Focus()
		return m, nil
	}

	if msg.String() == "tab" {
		m.acctInputs[m.acctIdx].Blur()
		m.acctIdx = (m.acctIdx + 1) % len(m.acctInputs)
		m.acctInputs[m.acctIdx].Focus()
		return m, nil
	}
	if msg.String() == "shift+tab" {
		m.acctInputs[m.acctIdx].Blur()
		m.acctIdx = (m.acctIdx - 1 + len(m.acctInputs)) % len(m.acctInputs)
		m.acctInputs[m.acctIdx].Focus()
		return m, nil
	}

	var cmd tea.Cmd
	m.acctInputs[m.acctIdx], cmd = m.acctInputs[m.acctIdx].Update(msg)
	return m, cmd
}

func (m learnModel) updateAmount(msg tea.KeyMsg) (learnModel, tea.Cmd) {
	if key.Matches(msg, keys.Escape) {
		m.state = learnAccounts
		m.acctInputs[m.acctIdx].Focus()
		m.err = nil
		return m, nil
	}
	if key.Matches(msg, keys.Enter) {
		amt := m.amountInput.Value()
		if amt == "" {
			m.err = fmt.Errorf("amount is required")
			return m, nil
		}
		cur := m.curOptions[m.currencyIdx]
		_, err := ledger.ToMinorUnits(amt, cur)
		if err != nil {
			m.err = fmt.Errorf("invalid amount: %v", err)
			return m, nil
		}
		m.err = nil
		m.state = learnCurrency
		return m, nil
	}
	var cmd tea.Cmd
	m.amountInput, cmd = m.amountInput.Update(msg)
	return m, cmd
}

func (m learnModel) updateCurrency(msg tea.KeyMsg) (learnModel, tea.Cmd) {
	if key.Matches(msg, keys.Escape) {
		m.state = learnAmount
		m.amountInput.Focus()
		return m, nil
	}
	switch {
	case key.Matches(msg, keys.Up):
		if m.currencyIdx > 0 {
			m.currencyIdx--
		}
	case key.Matches(msg, keys.Down):
		if m.currencyIdx < len(m.curOptions)-1 {
			m.currencyIdx++
		}
	case key.Matches(msg, keys.Enter):
		m.err = nil
		if m.isMultiCurrency() {
			// Move to destination amount/currency
			m.state = learnAmount2
			m.amountInput2 = textinput.New()
			m.amountInput2.Placeholder = "destination amount, e.g. 1100.00"
			m.amountInput2.CharLimit = 20
			m.amountInput2.Focus()
		} else {
			m.state = learnConfirm
		}
	}
	return m, nil
}

func (m learnModel) updateAmount2(msg tea.KeyMsg) (learnModel, tea.Cmd) {
	if key.Matches(msg, keys.Escape) {
		m.state = learnCurrency
		m.err = nil
		return m, nil
	}
	if key.Matches(msg, keys.Enter) {
		amt := m.amountInput2.Value()
		if amt == "" {
			m.err = fmt.Errorf("destination amount is required")
			return m, nil
		}
		cur := m.curOptions[m.currencyIdx2]
		_, err := ledger.ToMinorUnits(amt, cur)
		if err != nil {
			m.err = fmt.Errorf("invalid amount: %v", err)
			return m, nil
		}
		m.err = nil
		m.state = learnCurrency2
		return m, nil
	}
	var cmd tea.Cmd
	m.amountInput2, cmd = m.amountInput2.Update(msg)
	return m, cmd
}

func (m learnModel) updateCurrency2(msg tea.KeyMsg) (learnModel, tea.Cmd) {
	if key.Matches(msg, keys.Escape) {
		m.state = learnAmount2
		m.amountInput2.Focus()
		return m, nil
	}
	switch {
	case key.Matches(msg, keys.Up):
		if m.currencyIdx2 > 0 {
			m.currencyIdx2--
		}
	case key.Matches(msg, keys.Down):
		if m.currencyIdx2 < len(m.curOptions)-1 {
			m.currencyIdx2++
		}
	case key.Matches(msg, keys.Enter):
		m.err = nil
		m.state = learnConfirm
	}
	return m, nil
}

// buildEntries constructs the transaction entries from the template + user inputs.
func (m *learnModel) buildEntries() []ledger.Entry {
	tmpl := m.selected()
	if tmpl == nil {
		return nil
	}

	cur0 := m.curOptions[m.currencyIdx]
	minor0, _ := ledger.ToMinorUnits(m.amountInput.Value(), cur0)

	cur1 := cur0
	minor1 := minor0
	if tmpl.IsMultiCurrency() {
		cur1 = m.curOptions[m.currencyIdx2]
		minor1, _ = ledger.ToMinorUnits(m.amountInput2.Value(), cur1)
	}

	var entries []ledger.Entry
	for i, e := range tmpl.Entries {
		amt := minor0
		cur := cur0
		if e.Group == 1 {
			amt = minor1
			cur = cur1
		}
		if !e.IsDebit {
			amt = -amt
		}
		entries = append(entries, ledger.Entry{
			AccountID: strings.TrimSpace(m.acctInputs[i].Value()),
			Amount:    amt,
			Currency:  cur,
		})
	}
	return entries
}

func (m learnModel) updateConfirm(msg tea.KeyMsg, c *client.Client) (learnModel, tea.Cmd) {
	if key.Matches(msg, keys.Escape) {
		m.state = learnBrowse
		m.err = nil
		return m, nil
	}
	switch msg.String() {
	case "y", "Y", "enter":
		tmpl := m.templates[m.cursor]
		entries := m.buildEntries()
		txn := &ledger.Transaction{
			Description: tmpl.Name,
			Entries:     entries,
		}
		return m, func() tea.Msg {
			created, err := c.CreateTransaction(context.Background(), txn)
			return learnTxnCreatedMsg{txn: created, err: err}
		}
	case "n", "N":
		m.state = learnBrowse
		m.err = nil
	}
	return m, nil
}

func (m *learnModel) selected() *ledger.Template {
	if m.cursor >= 0 && m.cursor < len(m.templates) {
		return &m.templates[m.cursor]
	}
	return nil
}

func (m *learnModel) view() string {
	var b strings.Builder

	switch m.state {
	case learnBrowse:
		b.WriteString(titleStyle.Render("Learn — Transaction Templates"))
		b.WriteString("\n\n")

		if m.statusMsg != "" {
			b.WriteString(successStyle.Render("  "+m.statusMsg) + "\n\n")
		}

		maxRows := m.height - 4
		if maxRows < 5 {
			maxRows = 12
		}

		start := 0
		if m.cursor >= maxRows {
			start = m.cursor - maxRows + 1
		}

		for i := start; i < len(m.templates) && i < start+maxRows; i++ {
			t := m.templates[i]
			if i == m.cursor {
				b.WriteString(selectedStyle.Render(fmt.Sprintf("  > %-28s", t.Name)))
				b.WriteString("\n")
				b.WriteString(dimStyle.Render("    "+t.Description) + "\n")
				for _, e := range t.Entries {
					tag := "DR"
					style := debitStyle
					if !e.IsDebit {
						tag = "CR"
						style = creditStyle
					}
					defaultAcct := ledger.DefaultAccountForCoA(e.CoACode)
					b.WriteString(style.Render(fmt.Sprintf("      %s  CoA %d → %s  %s", tag, e.CoACode, defaultAcct, e.Role)) + "\n")
				}
				b.WriteString("\n")
			} else {
				b.WriteString(fmt.Sprintf("    %-28s\n", t.Name))
			}
		}

		b.WriteString(dimStyle.Render("\n  Enter to execute template"))

	case learnAccounts, learnAmount, learnCurrency, learnAmount2, learnCurrency2, learnConfirm:
		tmpl := m.selected()
		if tmpl == nil {
			return ""
		}

		b.WriteString(titleStyle.Render(tmpl.Name))
		b.WriteString("\n\n")

		b.WriteString("  " + dimStyle.Render(tmpl.Description) + "\n\n")

		// Show the journal entry pattern
		b.WriteString("  " + headerStyle.Render(fmt.Sprintf("%-4s  %-6s  %-14s  %s", "TYPE", "CoA", "ACCOUNT", "ROLE")) + "\n")
		for i, e := range tmpl.Entries {
			tag := "DR"
			style := debitStyle
			if !e.IsDebit {
				tag = "CR"
				style = creditStyle
			}
			acctID := ledger.DefaultAccountForCoA(e.CoACode)
			if m.acctInputs != nil && i < len(m.acctInputs) {
				acctID = m.acctInputs[i].Value()
			}
			groupLabel := ""
			if tmpl.IsMultiCurrency() {
				if e.Group == 0 {
					groupLabel = " [source]"
				} else {
					groupLabel = " [dest]"
				}
			}
			b.WriteString(style.Render(fmt.Sprintf("  %-4s  %-6d  %-14s  %s%s", tag, e.CoACode, acctID, e.Role, groupLabel)) + "\n")
		}
		b.WriteString("\n")

		switch m.state {
		case learnAccounts:
			b.WriteString("  Edit accounts (tab to switch, enter to proceed):\n\n")
			for i, e := range tmpl.Entries {
				tag := "DR"
				if !e.IsDebit {
					tag = "CR"
				}
				prefix := "  "
				if i == m.acctIdx {
					prefix = "> "
				}
				b.WriteString(fmt.Sprintf("  %s %s CoA %d — %s\n", prefix, tag, e.CoACode, e.Role))
				b.WriteString("     " + m.acctInputs[i].View() + "\n")
			}

			if len(m.accounts) > 0 {
				b.WriteString("\n  " + dimStyle.Render("Available accounts:") + "\n")
				for _, a := range m.accounts {
					b.WriteString(dimStyle.Render(fmt.Sprintf("    %-14s  %s  (%s)", a.ID, a.Name, a.Currency)) + "\n")
				}
			}

		case learnAmount:
			if tmpl.IsMultiCurrency() {
				b.WriteString("  Enter SOURCE amount:\n\n")
			} else {
				b.WriteString("  Enter amount:\n\n")
			}
			b.WriteString("  " + m.amountInput.View() + "\n")

		case learnCurrency:
			if tmpl.IsMultiCurrency() {
				b.WriteString(fmt.Sprintf("  Source amount: %s\n", m.amountInput.Value()))
				b.WriteString("  Select SOURCE currency:\n\n")
			} else {
				b.WriteString(fmt.Sprintf("  Amount: %s\n", m.amountInput.Value()))
				b.WriteString("  Select currency:\n\n")
			}
			m.viewCurrencyPicker(&b, m.currencyIdx)

		case learnAmount2:
			srcCur := m.curOptions[m.currencyIdx]
			b.WriteString(fmt.Sprintf("  Source: %s %s\n", m.amountInput.Value(), srcCur))
			b.WriteString("  Enter DESTINATION amount:\n\n")
			b.WriteString("  " + m.amountInput2.View() + "\n")

		case learnCurrency2:
			srcCur := m.curOptions[m.currencyIdx]
			b.WriteString(fmt.Sprintf("  Source: %s %s\n", m.amountInput.Value(), srcCur))
			b.WriteString(fmt.Sprintf("  Destination amount: %s\n", m.amountInput2.Value()))
			b.WriteString("  Select DESTINATION currency:\n\n")
			m.viewCurrencyPicker(&b, m.currencyIdx2)

		case learnConfirm:
			entries := m.buildEntries()
			b.WriteString("  Review:\n\n")

			var summary strings.Builder
			summary.WriteString(fmt.Sprintf("%s %s\n\n", labelStyle.Render("Description:"), tmpl.Name))
			for _, e := range entries {
				tag := "DR"
				amt := e.Amount
				if amt < 0 {
					tag = "CR"
					amt = -amt
				}
				formatted := ledger.FormatAmount(amt, e.Currency)
				summary.WriteString(fmt.Sprintf("  %s  %-14s  %s %s\n", tag, e.AccountID, formatted, e.Currency))
			}
			b.WriteString(boxStyle.Render(summary.String()))
			b.WriteString("\n\n")

			// Ratio impact
			if m.ratios != nil {
				acctMap := make(map[string]ledger.Account, len(m.accounts))
				for _, a := range m.accounts {
					acctMap[a.ID] = a
				}
				projected := ledger.ProjectRatios(m.ratios, entries, acctMap)
				b.WriteString(RenderRatioImpact(m.ratios, projected))
				b.WriteString("\n")
			}

			b.WriteString("  Post this transaction? (y/n)\n")
		}

		b.WriteString("\n" + dimStyle.Render("  ESC to go back"))
	}

	if m.err != nil {
		b.WriteString("\n" + errorStyle.Render("  Error: "+m.err.Error()) + "\n")
	}

	return b.String()
}

func (m *learnModel) viewCurrencyPicker(b *strings.Builder, idx int) {
	start := idx - 3
	if start < 0 {
		start = 0
	}
	end := start + 7
	if end > len(m.curOptions) {
		end = len(m.curOptions)
	}
	for i := start; i < end; i++ {
		code := m.curOptions[i]
		cur := ledger.Currencies[code]
		label := fmt.Sprintf("%s - %s", code, cur.Name)
		if i == idx {
			b.WriteString(selectedStyle.Render("  > "+label) + "\n")
		} else {
			b.WriteString(fmt.Sprintf("    %s\n", label))
		}
	}
}
