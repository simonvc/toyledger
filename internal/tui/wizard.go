package tui

import (
	"context"
	"fmt"
	"regexp"
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

var bankNameRe = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

type accountCreatedMsg struct {
	account *ledger.Account
	err     error
}

type nextCustomerIDMsg struct {
	nextID string
	err    error
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

	autoID    string // auto-generated ID for code 2020 accounts
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

// codeVal returns the parsed IFRS code (0 if unparsed).
func (m wizardModel) codeVal() int {
	code, _ := strconv.Atoi(m.code.Value())
	return code
}

// finalID constructs the complete account ID based on code-specific rules.
func (m wizardModel) finalID() string {
	ccy := strings.ToLower(m.curOptions[m.currency])
	switch m.codeVal() {
	case 1010, 1060:
		return "<" + strings.ToLower(m.id.Value()) + ":" + ccy + ">"
	case 2010:
		return ">" + strings.ToLower(m.id.Value()) + ":" + ccy + "<"
	case 2020:
		return m.autoID
	default:
		return m.id.Value()
	}
}

// stepProgress returns the "Step N of M" string, accounting for reordered flows.
func (m wizardModel) stepProgress() string {
	code := m.codeVal()
	var stepNum, total int
	switch {
	case code == 2020:
		total = 5
		switch m.step {
		case stepCategory:
			stepNum = 1
		case stepCode:
			stepNum = 2
		case stepCurrency:
			stepNum = 3
		case stepName:
			stepNum = 4
		case stepConfirm:
			stepNum = 5
		}
	case code == 1010 || code == 1060 || code == 2010:
		total = 6
		switch m.step {
		case stepCategory:
			stepNum = 1
		case stepCode:
			stepNum = 2
		case stepCurrency:
			stepNum = 3
		case stepID:
			stepNum = 4
		case stepName:
			stepNum = 5
		case stepConfirm:
			stepNum = 6
		}
	default:
		total = 6
		stepNum = int(m.step) + 1
	}
	return fmt.Sprintf("Step %d of %d", stepNum, total)
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

	case nextCustomerIDMsg:
		if msg.err != nil {
			m.autoID = "acc_1"
		} else {
			m.autoID = msg.nextID
		}
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

		switch code {
		case 1010, 1060, 2010:
			// Correspondent/reserve accounts: pick currency first, then bank name
			m.step = stepCurrency
			return m, nil
		case 2020:
			// Customer accounts: pick currency first, ID auto-generated
			m.step = stepCurrency
			return m, func() tea.Msg {
				accounts, err := c.ListAccounts(context.Background(), "", nil)
				if err != nil {
					return nextCustomerIDMsg{err: err}
				}
				maxN := 0
				for _, acct := range accounts {
					if acct.Code == 2020 {
						var n int
						if _, e := fmt.Sscanf(acct.ID, "acc_%d", &n); e == nil && n > maxN {
							maxN = n
						}
					}
				}
				return nextCustomerIDMsg{nextID: fmt.Sprintf("acc_%d", maxN+1)}
			}
		default:
			m.step = stepID
			m.id.SetValue(strconv.Itoa(code))
			m.id.Focus()
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.code, cmd = m.code.Update(msg)
	return m, cmd
}

func (m wizardModel) updateID(msg tea.KeyMsg, c *client.Client) (wizardModel, tea.Cmd) {
	if key.Matches(msg, keys.Enter) {
		val := m.id.Value()
		if val == "" {
			m.err = fmt.Errorf("ID is required")
			return m, nil
		}

		code := m.codeVal()
		if code == 1010 || code == 1060 || code == 2010 {
			if !bankNameRe.MatchString(val) {
				m.err = fmt.Errorf("bank name must be alphanumeric, hyphens, or underscores")
				return m, nil
			}
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

		code := m.codeVal()
		switch code {
		case 1010, 1060, 2010, 2020:
			// Currency was already selected; go straight to confirm
			m.step = stepConfirm
		default:
			m.step = stepCurrency
		}
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
		code := m.codeVal()
		switch code {
		case 1010, 1060:
			m.step = stepID
			m.id.SetValue("")
			m.id.Placeholder = "bank name e.g. nbg"
			m.id.Focus()
		case 2010:
			m.step = stepID
			m.id.SetValue("")
			m.id.Placeholder = "bank name e.g. jpmorgan"
			m.id.Focus()
		case 2020:
			m.step = stepName
			m.name.Focus()
		default:
			m.step = stepConfirm
		}
	}
	return m, nil
}

func (m wizardModel) updateConfirm(msg tea.KeyMsg, c *client.Client) (wizardModel, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter":
		code := m.codeVal()
		id := m.finalID()
		acct := &ledger.Account{
			ID:       id,
			Name:     m.name.Value(),
			Code:     code,
			Category: m.category,
			Currency: m.curOptions[m.currency],
			IsSystem: strings.HasPrefix(id, "~"),
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

	b.WriteString(dimStyle.Render(m.stepProgress()))
	b.WriteString("\n\n")

	code := m.codeVal()

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

		// Show all IFRS codes filtered by typed prefix
		typed := m.code.Value()
		b.WriteString("\n" + dimStyle.Render("  IFRS Chart of Accounts:") + "\n")
		shown := 0
		for _, entry := range ledger.AllChartEntries() {
			codeStr := strconv.Itoa(entry.Code)
			if typed != "" && !strings.HasPrefix(codeStr, typed) {
				continue
			}
			inRange := entry.Code >= low && entry.Code <= high
			marker := "  "
			if inRange {
				marker = "▸ "
			}
			line := fmt.Sprintf("    %s%d  %-30s  %s", marker, entry.Code, entry.Name, entry.Description)
			if !inRange {
				line = dimStyle.Render(line)
			}
			b.WriteString(line + "\n")
			shown++
		}
		if shown == 0 && typed != "" {
			b.WriteString(dimStyle.Render("    (no matching codes)") + "\n")
		}

	case stepID:
		switch code {
		case 1010, 1060:
			ccy := strings.ToLower(m.curOptions[m.currency])
			b.WriteString(fmt.Sprintf("  Code: %s | Currency: %s\n", m.code.Value(), m.curOptions[m.currency]))
			b.WriteString("  Enter bank name:\n\n")
			b.WriteString("  " + m.id.View() + "\n")
			preview := "<" + strings.ToLower(m.id.Value()) + ":" + ccy + ">"
			if m.id.Value() == "" {
				preview = "<bank:" + ccy + ">"
			}
			var hintLabel string
			if code == 1060 {
				hintLabel = fmt.Sprintf("RESERVES (1060): ID will be %s\n\n", preview) +
					"The < > format identifies where reserves\n" +
					"are held and in which currency."
			} else {
				hintLabel = fmt.Sprintf("NOSTRO (1010): ID will be %s\n\n", preview) +
					"The < > arrows represent money flowing\n" +
					"OUT of the bank to the correspondent."
			}
			b.WriteString("\n" + hintBoxStyle.Render(hintLabel) + "\n")
		case 2010:
			ccy := strings.ToLower(m.curOptions[m.currency])
			b.WriteString(fmt.Sprintf("  Code: %s | Currency: %s\n", m.code.Value(), m.curOptions[m.currency]))
			b.WriteString("  Enter bank name:\n\n")
			b.WriteString("  " + m.id.View() + "\n")
			preview := ">" + strings.ToLower(m.id.Value()) + ":" + ccy + "<"
			if m.id.Value() == "" {
				preview = ">bank:" + ccy + "<"
			}
			b.WriteString("\n" + hintBoxStyle.Render(
				fmt.Sprintf("VOSTRO (2010): ID will be %s\n\n", preview)+
					"The > < arrows represent money flowing\n"+
					"IN to the bank from the correspondent.",
			) + "\n")
		default:
			b.WriteString(fmt.Sprintf("  Code: %s\n", m.code.Value()))
			b.WriteString("  Enter account ID:\n\n")
			b.WriteString("  " + m.id.View() + "\n")
			b.WriteString("\n" + dimStyle.Render("  Use ~ prefix for system/internal accounts") + "\n")
		}

	case stepName:
		switch {
		case code == 1010 || code == 1060 || code == 2010:
			b.WriteString(fmt.Sprintf("  Code: %s | ID: %s\n", m.code.Value(), m.finalID()))
		case code == 2020:
			idLabel := m.autoID
			if idLabel == "" {
				idLabel = "acc_..."
			}
			b.WriteString(fmt.Sprintf("  Code: %s | ID: %s | Currency: %s\n", m.code.Value(), idLabel, m.curOptions[m.currency]))
		default:
			b.WriteString(fmt.Sprintf("  Code: %s | ID: %s\n", m.code.Value(), m.id.Value()))
		}
		b.WriteString("  Enter account name:\n\n")
		b.WriteString("  " + m.name.View() + "\n")

	case stepCurrency:
		switch {
		case code == 1010 || code == 1060 || code == 2010 || code == 2020:
			b.WriteString(fmt.Sprintf("  Code: %s\n", m.code.Value()))
		default:
			b.WriteString(fmt.Sprintf("  Code: %s | ID: %s | Name: %s\n", m.code.Value(), m.id.Value(), m.name.Value()))
		}
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
		id := m.finalID()
		b.WriteString("  Review and confirm:\n\n")
		b.WriteString(boxStyle.Render(fmt.Sprintf(
			"%s %s\n%s %s\n%s %s\n%s %s\n%s %s",
			labelStyle.Render("Category:"), ledger.CategoryLabel(m.category),
			labelStyle.Render("Code:"), m.code.Value(),
			labelStyle.Render("ID:"), id,
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
