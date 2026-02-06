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

	// Account editing state
	acctInputs []textinput.Model // one text input per template entry
	acctIdx    int               // which entry we're editing
	accounts   []ledger.Account  // loaded accounts for reference
}

func (m *learnModel) init() {
	m.templates = ledger.Templates
	m.curOptions = ledger.CurrencyCodes()
	// Default to USD
	for i, c := range m.curOptions {
		if c == "USD" {
			m.currencyIdx = i
			break
		}
	}
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

	case tea.KeyMsg:
		switch m.state {
		case learnBrowse:
			return m.updateBrowse(msg, c)
		case learnAccounts:
			return m.updateAccounts(msg)
		case learnAmount:
			return m.updateAmount(msg)
		case learnCurrency:
			return m.updateCurrency(msg)
		case learnConfirm:
			return m.updateConfirm(msg, c)
		}
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
		// Set up account text inputs pre-filled with defaults
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
		// Load accounts list for reference
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
		// Validate current input is not empty
		val := strings.TrimSpace(m.acctInputs[m.acctIdx].Value())
		if val == "" {
			m.err = fmt.Errorf("account ID is required")
			return m, nil
		}
		m.err = nil
		m.acctInputs[m.acctIdx].Blur()

		// Move to next entry or proceed to amount
		if m.acctIdx < len(m.acctInputs)-1 {
			m.acctIdx++
			m.acctInputs[m.acctIdx].Focus()
			return m, nil
		}
		// All accounts set, move to amount
		m.state = learnAmount
		m.amountInput = textinput.New()
		m.amountInput.Placeholder = "e.g. 1000.00"
		m.amountInput.CharLimit = 20
		m.amountInput.Focus()
		return m, nil
	}

	// Tab moves between account inputs
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
		// Go back to accounts
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
		m.state = learnConfirm
	}
	return m, nil
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
		cur := m.curOptions[m.currencyIdx]
		minor, _ := ledger.ToMinorUnits(m.amountInput.Value(), cur)

		txn := &ledger.Transaction{
			Description: tmpl.Name,
		}
		for i, e := range tmpl.Entries {
			amt := minor
			if !e.IsDebit {
				amt = -amt
			}
			accountID := strings.TrimSpace(m.acctInputs[i].Value())
			txn.Entries = append(txn.Entries, ledger.Entry{
				AccountID: accountID,
				Amount:    amt,
				Currency:  cur,
			})
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

	case learnAccounts, learnAmount, learnCurrency, learnConfirm:
		tmpl := m.selected()
		if tmpl == nil {
			return ""
		}

		b.WriteString(titleStyle.Render(tmpl.Name))
		b.WriteString("\n\n")

		// Explanation
		b.WriteString("  " + dimStyle.Render(tmpl.Description) + "\n\n")

		// Show the journal entry pattern with CoA codes
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
			b.WriteString(style.Render(fmt.Sprintf("  %-4s  %-6d  %-14s  %s", tag, e.CoACode, acctID, e.Role)) + "\n")
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

			// Show available accounts for reference
			if len(m.accounts) > 0 {
				b.WriteString("\n  " + dimStyle.Render("Available accounts:") + "\n")
				for _, a := range m.accounts {
					b.WriteString(dimStyle.Render(fmt.Sprintf("    %-14s  %s  (%s)", a.ID, a.Name, a.Currency)) + "\n")
				}
			}

		case learnAmount:
			b.WriteString("  Enter amount:\n\n")
			b.WriteString("  " + m.amountInput.View() + "\n")

		case learnCurrency:
			b.WriteString(fmt.Sprintf("  Amount: %s\n", m.amountInput.Value()))
			b.WriteString("  Select currency:\n\n")

			start := m.currencyIdx - 3
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
				if i == m.currencyIdx {
					b.WriteString(selectedStyle.Render("  > "+label) + "\n")
				} else {
					b.WriteString(fmt.Sprintf("    %s\n", label))
				}
			}

		case learnConfirm:
			cur := m.curOptions[m.currencyIdx]
			minor, _ := ledger.ToMinorUnits(m.amountInput.Value(), cur)
			formatted := ledger.FormatAmount(minor, cur)

			b.WriteString("  Review:\n\n")

			var summary strings.Builder
			summary.WriteString(fmt.Sprintf("%s %s\n", labelStyle.Render("Description:"), tmpl.Name))
			summary.WriteString(fmt.Sprintf("%s %s %s\n\n", labelStyle.Render("Amount:"), formatted, cur))
			for i, e := range tmpl.Entries {
				tag := "DR"
				if !e.IsDebit {
					tag = "CR"
				}
				acctID := strings.TrimSpace(m.acctInputs[i].Value())
				summary.WriteString(fmt.Sprintf("  %s  %-14s  %s %s\n", tag, acctID, formatted, cur))
			}
			b.WriteString(boxStyle.Render(summary.String()))
			b.WriteString("\n\n  Post this transaction? (y/n)\n")
		}

		b.WriteString("\n" + dimStyle.Render("  ESC to go back"))
	}

	if m.err != nil {
		b.WriteString("\n" + errorStyle.Render("  Error: "+m.err.Error()) + "\n")
	}

	return b.String()
}
