package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/simonvc/miniledger/internal/client"
	"github.com/simonvc/miniledger/internal/ledger"
)

type wizardStep int

const (
	stepCategory wizardStep = iota
	stepCode
	stepID
	stepName
	stepCurrency
	stepConfirm
)

type accountCreatedMsg struct {
	account *ledger.Account
	err     error
}

type wizardModel struct {
	step       wizardStep
	category   ledger.Category
	catCursor  int
	code       textinput.Model
	id         textinput.Model
	name       textinput.Model
	currency   int // index into currency list
	curOptions []string

	err       error
	done      bool
	cancelled bool
	statusMsg string
	width     int
}

func newWizard() wizardModel {
	codeInput := textinput.New()
	codeInput.Placeholder = "e.g. 1060"
	codeInput.CharLimit = 4

	idInput := textinput.New()
	idInput.Placeholder = "e.g. 1060 or ~myaccount"
	idInput.CharLimit = 30

	nameInput := textinput.New()
	nameInput.Placeholder = "e.g. Petty Cash"
	nameInput.CharLimit = 60

	return wizardModel{
		step:       stepCategory,
		code:       codeInput,
		id:         idInput,
		name:       nameInput,
		curOptions: ledger.CurrencyCodes(),
	}
}

func (m wizardModel) update(msg tea.Msg, c *client.Client) (wizardModel, tea.Cmd) {
	switch msg := msg.(type) {
	case accountCreatedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.step = stepConfirm
			return m, nil
		}
		m.done = true
		m.statusMsg = fmt.Sprintf("Account %s created successfully!", msg.account.ID)
		return m, nil

	case tea.KeyMsg:
		// ESC cancels at any step
		if key.Matches(msg, keys.Escape) {
			m.cancelled = true
			return m, nil
		}

		switch m.step {
		case stepCategory:
			return m.updateCategory(msg)
		case stepCode:
			return m.updateCode(msg, c)
		case stepID:
			return m.updateID(msg, c)
		case stepName:
			return m.updateName(msg, c)
		case stepCurrency:
			return m.updateCurrency(msg, c)
		case stepConfirm:
			return m.updateConfirm(msg, c)
		}
	}
	return m, nil
}

func (m wizardModel) updateCategory(msg tea.KeyMsg) (wizardModel, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Up):
		if m.catCursor > 0 {
			m.catCursor--
		}
	case key.Matches(msg, keys.Down):
		if m.catCursor < len(ledger.AllCategories)-1 {
			m.catCursor++
		}
	case key.Matches(msg, keys.Enter):
		m.category = ledger.AllCategories[m.catCursor]
		m.step = stepCode
		low, high := ledger.CodeRange(m.category)
		m.code.Placeholder = fmt.Sprintf("%d-%d", low, high)
		m.code.Focus()
		m.err = nil
	}
	return m, nil
}

func (m wizardModel) updateCode(msg tea.KeyMsg, c *client.Client) (wizardModel, tea.Cmd) {
	if key.Matches(msg, keys.Enter) {
		code, err := strconv.Atoi(m.code.Value())
		if err != nil {
			m.err = fmt.Errorf("code must be a number")
			return m, nil
		}
		cat, err := ledger.CategoryForCode(code)
		if err != nil || cat != m.category {
			low, high := ledger.CodeRange(m.category)
			m.err = fmt.Errorf("code must be between %d-%d for %s", low, high, m.category)
			return m, nil
		}
		m.err = nil
		m.step = stepID
		m.id.SetValue(strconv.Itoa(code))
		m.id.Focus()
		return m, nil
	}

	var cmd tea.Cmd
	m.code, cmd = m.code.Update(msg)
	return m, cmd
}

func (m wizardModel) updateID(msg tea.KeyMsg, c *client.Client) (wizardModel, tea.Cmd) {
	if key.Matches(msg, keys.Enter) {
		if m.id.Value() == "" {
			m.err = fmt.Errorf("ID is required")
			return m, nil
		}
		m.err = nil
		m.step = stepName
		m.name.Focus()
		return m, nil
	}

	var cmd tea.Cmd
	m.id, cmd = m.id.Update(msg)
	return m, cmd
}

func (m wizardModel) updateName(msg tea.KeyMsg, c *client.Client) (wizardModel, tea.Cmd) {
	if key.Matches(msg, keys.Enter) {
		if m.name.Value() == "" {
			m.err = fmt.Errorf("name is required")
			return m, nil
		}
		m.err = nil
		m.step = stepCurrency
		return m, nil
	}

	var cmd tea.Cmd
	m.name, cmd = m.name.Update(msg)
	return m, cmd
}

func (m wizardModel) updateCurrency(msg tea.KeyMsg, c *client.Client) (wizardModel, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Up):
		if m.currency > 0 {
			m.currency--
		}
	case key.Matches(msg, keys.Down):
		if m.currency < len(m.curOptions)-1 {
			m.currency++
		}
	case key.Matches(msg, keys.Enter):
		m.err = nil
		m.step = stepConfirm
	}
	return m, nil
}

func (m wizardModel) updateConfirm(msg tea.KeyMsg, c *client.Client) (wizardModel, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter":
		code, _ := strconv.Atoi(m.code.Value())
		acct := &ledger.Account{
			ID:       m.id.Value(),
			Name:     m.name.Value(),
			Code:     code,
			Category: m.category,
			Currency: m.curOptions[m.currency],
			IsSystem: strings.HasPrefix(m.id.Value(), "~"),
		}
		return m, func() tea.Msg {
			created, err := c.CreateAccount(context.Background(), acct)
			return accountCreatedMsg{account: created, err: err}
		}
	case "n", "N":
		m.cancelled = true
	}
	return m, nil
}

func (m *wizardModel) view() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Create Account Wizard"))
	b.WriteString("\n\n")

	progress := fmt.Sprintf("Step %d of 6", int(m.step)+1)
	b.WriteString(dimStyle.Render(progress))
	b.WriteString("\n\n")

	switch m.step {
	case stepCategory:
		b.WriteString("  Select account category (IFRS):\n\n")
		for i, cat := range ledger.AllCategories {
			low, high := ledger.CodeRange(cat)
			desc := fmt.Sprintf("%s (codes %d-%d)", ledger.CategoryLabel(cat), low, high)
			if i == m.catCursor {
				b.WriteString(selectedStyle.Render("  > "+desc) + "\n")
			} else {
				b.WriteString(fmt.Sprintf("    %s\n", desc))
			}
		}

	case stepCode:
		low, high := ledger.CodeRange(m.category)
		b.WriteString(fmt.Sprintf("  Category: %s\n", ledger.CategoryLabel(m.category)))
		b.WriteString(fmt.Sprintf("  Enter account code (%d-%d):\n\n", low, high))
		b.WriteString("  " + m.code.View() + "\n")

		// Show predefined accounts in this range as reference
		b.WriteString("\n" + dimStyle.Render("  Existing IFRS codes in this category:") + "\n")
		for _, pa := range ledger.PredefinedAccounts {
			if pa.Code >= low && pa.Code <= high {
				b.WriteString(dimStyle.Render(fmt.Sprintf("    %d - %s", pa.Code, pa.Name)) + "\n")
			}
		}

	case stepID:
		b.WriteString(fmt.Sprintf("  Code: %s\n", m.code.Value()))
		b.WriteString("  Enter account ID:\n\n")
		b.WriteString("  " + m.id.View() + "\n")
		b.WriteString("\n" + dimStyle.Render("  Use ~ prefix for system/internal accounts") + "\n")

	case stepName:
		b.WriteString(fmt.Sprintf("  Code: %s | ID: %s\n", m.code.Value(), m.id.Value()))
		b.WriteString("  Enter account name:\n\n")
		b.WriteString("  " + m.name.View() + "\n")

	case stepCurrency:
		b.WriteString(fmt.Sprintf("  Code: %s | ID: %s | Name: %s\n", m.code.Value(), m.id.Value(), m.name.Value()))
		b.WriteString("  Select currency:\n\n")

		// Show a window of currencies around the cursor
		start := m.currency - 5
		if start < 0 {
			start = 0
		}
		end := start + 11
		if end > len(m.curOptions) {
			end = len(m.curOptions)
		}

		for i := start; i < end; i++ {
			code := m.curOptions[i]
			cur := ledger.Currencies[code]
			label := fmt.Sprintf("%s - %s", code, cur.Name)
			if i == m.currency {
				b.WriteString(selectedStyle.Render("  > "+label) + "\n")
			} else {
				b.WriteString(fmt.Sprintf("    %s\n", label))
			}
		}

	case stepConfirm:
		b.WriteString("  Review and confirm:\n\n")
		b.WriteString(boxStyle.Render(fmt.Sprintf(
			"%s %s\n%s %s\n%s %s\n%s %s\n%s %s",
			labelStyle.Render("Category:"), ledger.CategoryLabel(m.category),
			labelStyle.Render("Code:"), m.code.Value(),
			labelStyle.Render("ID:"), m.id.Value(),
			labelStyle.Render("Name:"), m.name.Value(),
			labelStyle.Render("Currency:"), m.curOptions[m.currency],
		)))
		b.WriteString("\n\n")
		b.WriteString("  Create this account? (y/n)\n")
	}

	if m.err != nil {
		b.WriteString("\n" + errorStyle.Render("  Error: "+m.err.Error()) + "\n")
	}

	b.WriteString("\n" + dimStyle.Render("  ESC to cancel"))
	return b.String()
}
