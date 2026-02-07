package tui

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/simonvc/miniledger/internal/client"
	"github.com/simonvc/miniledger/internal/ledger"
)

type fxStep int

const (
	fxStepLanding fxStep = iota
	fxStepSrcAcct
	fxStepDestAcct
	fxStepIntention
	fxStepRate
	fxStepConfirm
)

// otcFXLoadedMsg is sent when accounts are loaded for the OTC FX tab.
type otcFXLoadedMsg struct {
	accounts []ledger.Account
	err      error
}

// otcFXTxnCreatedMsg is sent when the FX transaction is created.
type otcFXTxnCreatedMsg struct {
	txn *ledger.Transaction
	err error
}

type otcFXModel struct {
	step   fxStep
	width  int
	height int

	// Step 1: Source account
	srcAcctInput textinput.Model
	srcAcctID    string
	srcCurrency  string

	// Step 2: Dest account
	destAcctInput textinput.Model
	destAcctID    string
	destCurrency  string

	// Step 3: Intention
	intentionInput textinput.Model
	intentionVerb  string // "give" or "get"
	intentionAmt   int64  // minor units
	intentionCur   string // currency of the amount

	// Step 4: Rate
	spreadBps int
	exactRate float64
	rateInput textinput.Model
	rateExact bool

	// Shared
	accounts  []ledger.Account
	err       error
	done      bool
	cancelled bool
	statusMsg string
}

func newOTCFX() otcFXModel {
	srcAcct := textinput.New()
	srcAcct.Placeholder = "account ID (e.g. alice-usd)"
	srcAcct.CharLimit = 30

	destAcct := textinput.New()
	destAcct.Placeholder = "account ID (e.g. alice-eur)"
	destAcct.CharLimit = 30

	intentInput := textinput.New()
	intentInput.Placeholder = "give 100 or get 500"
	intentInput.CharLimit = 40

	rateInput := textinput.New()
	rateInput.Placeholder = "exact rate"
	rateInput.CharLimit = 20

	return otcFXModel{
		step:           fxStepLanding,
		srcAcctInput:   srcAcct,
		destAcctInput:  destAcct,
		intentionInput: intentInput,
		rateInput:      rateInput,
	}
}

func (m *otcFXModel) init(c *client.Client) tea.Cmd {
	return func() tea.Msg {
		accounts, err := c.ListAccounts(context.Background(), "", nil)
		return otcFXLoadedMsg{accounts: accounts, err: err}
	}
}

func (m *otcFXModel) sourceCur() string { return m.srcCurrency }
func (m *otcFXModel) destCur() string   { return m.destCurrency }

func (m *otcFXModel) midRate() float64 {
	if m.srcCurrency == "" || m.destCurrency == "" {
		return 0
	}
	src := ledger.FXRatesToGEL[m.srcCurrency]
	dst := ledger.FXRatesToGEL[m.destCurrency]
	if dst == 0 {
		return 0
	}
	return src / dst
}

func (m *otcFXModel) effectiveRate() float64 {
	if m.rateExact && m.exactRate > 0 {
		return m.exactRate
	}
	return m.midRate() * (1 - float64(m.spreadBps)/10000.0)
}

func (m *otcFXModel) spreadFromRate(rate float64) float64 {
	mid := m.midRate()
	if mid == 0 {
		return 0
	}
	return (1 - rate/mid) * 10000.0
}

func (m *otcFXModel) customerGives() int64 {
	srcCur := m.sourceCur()
	dstCur := m.destCur()
	srcMul := math.Pow10(ledger.Currencies[srcCur].Exponent)
	dstMul := math.Pow10(ledger.Currencies[dstCur].Exponent)
	rate := m.effectiveRate()

	if m.intentionVerb == "give" {
		return m.intentionAmt
	}
	if rate == 0 {
		return 0
	}
	return int64(math.Round(float64(m.intentionAmt) / rate * srcMul / dstMul))
}

func (m *otcFXModel) customerGets() int64 {
	srcCur := m.sourceCur()
	dstCur := m.destCur()
	srcMul := math.Pow10(ledger.Currencies[srcCur].Exponent)
	dstMul := math.Pow10(ledger.Currencies[dstCur].Exponent)
	rate := m.effectiveRate()

	if m.intentionVerb == "get" {
		return m.intentionAmt
	}
	return int64(math.Round(float64(m.intentionAmt) * rate * dstMul / srcMul))
}

func (m *otcFXModel) bankPnLGEL() int64 {
	return ledger.ToGEL(m.customerGives(), m.sourceCur()) - ledger.ToGEL(m.customerGets(), m.destCur())
}

func (m otcFXModel) update(msg tea.Msg, c *client.Client) (otcFXModel, tea.Cmd) {
	switch msg := msg.(type) {
	case otcFXLoadedMsg:
		if msg.err == nil {
			m.accounts = msg.accounts
		}
		return m, nil

	case otcFXTxnCreatedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.done = true
		m.statusMsg = fmt.Sprintf("FX trade %s executed!", msg.txn.ID[:8])
		return m, nil

	case tea.KeyMsg:
		// ESC goes back one step; on landing, do nothing (normal tab nav handles it)
		if m.step > fxStepLanding && key.Matches(msg, keys.Escape) {
			m.step--
			m.err = nil
			switch m.step {
			case fxStepLanding:
				// back to landing, no focus needed
			case fxStepSrcAcct:
				m.srcAcctInput.Focus()
			case fxStepDestAcct:
				m.destAcctInput.Focus()
			case fxStepIntention:
				m.intentionInput.Focus()
			case fxStepRate:
				m.rateExact = false
				m.rateInput.Blur()
			}
			return m, nil
		}

		switch m.step {
		case fxStepLanding:
			return m.updateLanding(msg)
		case fxStepSrcAcct:
			return m.updateSrcAcct(msg)
		case fxStepDestAcct:
			return m.updateDestAcct(msg)
		case fxStepIntention:
			return m.updateIntention(msg)
		case fxStepRate:
			return m.updateRate(msg)
		case fxStepConfirm:
			return m.updateConfirm(msg, c)
		}
	}
	return m, nil
}

func (m otcFXModel) updateLanding(msg tea.KeyMsg) (otcFXModel, tea.Cmd) {
	if msg.String() == "f" {
		m.step = fxStepSrcAcct
		m.srcAcctInput.SetValue("")
		m.srcAcctInput.Focus()
		m.srcAcctID = ""
		m.srcCurrency = ""
		m.destAcctID = ""
		m.destCurrency = ""
		m.err = nil
	}
	return m, nil
}

func (m otcFXModel) updateSrcAcct(msg tea.KeyMsg) (otcFXModel, tea.Cmd) {
	if key.Matches(msg, keys.Enter) {
		id := strings.TrimSpace(m.srcAcctInput.Value())
		if id == "" {
			m.err = fmt.Errorf("account ID is required")
			return m, nil
		}
		for _, a := range m.accounts {
			if a.ID == id {
				if a.Currency == "*" {
					m.err = fmt.Errorf("cannot use wildcard-currency account")
					return m, nil
				}
				m.srcAcctID = id
				m.srcCurrency = a.Currency
				m.err = nil
				m.step = fxStepDestAcct
				m.srcAcctInput.Blur()
				m.destAcctInput.SetValue("")
				m.destAcctInput.Focus()
				return m, nil
			}
		}
		m.err = fmt.Errorf("account '%s' not found", id)
		return m, nil
	}
	var cmd tea.Cmd
	m.srcAcctInput, cmd = m.srcAcctInput.Update(msg)
	return m, cmd
}

func (m otcFXModel) updateDestAcct(msg tea.KeyMsg) (otcFXModel, tea.Cmd) {
	if key.Matches(msg, keys.Enter) {
		id := strings.TrimSpace(m.destAcctInput.Value())
		if id == "" {
			m.err = fmt.Errorf("account ID is required")
			return m, nil
		}
		for _, a := range m.accounts {
			if a.ID == id {
				if a.Currency == "*" {
					m.err = fmt.Errorf("cannot use wildcard-currency account")
					return m, nil
				}
				if a.Currency == m.srcCurrency {
					m.err = fmt.Errorf("dest must be different currency (source is %s)", m.srcCurrency)
					return m, nil
				}
				m.destAcctID = id
				m.destCurrency = a.Currency
				m.err = nil
				m.step = fxStepIntention
				m.destAcctInput.Blur()
				m.intentionInput.SetValue("")
				m.intentionInput.Focus()
				return m, nil
			}
		}
		m.err = fmt.Errorf("account '%s' not found", id)
		return m, nil
	}
	var cmd tea.Cmd
	m.destAcctInput, cmd = m.destAcctInput.Update(msg)
	return m, cmd
}

func (m otcFXModel) updateIntention(msg tea.KeyMsg) (otcFXModel, tea.Cmd) {
	if key.Matches(msg, keys.Enter) {
		val := strings.TrimSpace(m.intentionInput.Value())
		parts := strings.Fields(val)
		if len(parts) < 2 || len(parts) > 3 {
			m.err = fmt.Errorf("enter: give <amount> [%s] or get <amount> [%s]", m.srcCurrency, m.destCurrency)
			return m, nil
		}
		verb := strings.ToLower(parts[0])
		if verb != "give" && verb != "get" {
			m.err = fmt.Errorf("first word must be 'give' or 'get'")
			return m, nil
		}

		expectedCur := m.srcCurrency
		if verb == "get" {
			expectedCur = m.destCurrency
		}

		// If 3 parts, validate currency matches
		if len(parts) == 3 {
			cur := strings.ToUpper(parts[2])
			if cur != expectedCur {
				m.err = fmt.Errorf("currency must be %s for '%s' (got %s)", expectedCur, verb, cur)
				return m, nil
			}
		}

		amt, err := ledger.ToMinorUnits(parts[1], expectedCur)
		if err != nil {
			m.err = fmt.Errorf("invalid amount: %v", err)
			return m, nil
		}
		if amt <= 0 {
			m.err = fmt.Errorf("amount must be positive")
			return m, nil
		}

		m.intentionVerb = verb
		m.intentionAmt = amt
		m.intentionCur = expectedCur
		m.err = nil
		m.step = fxStepRate
		m.spreadBps = 0
		m.rateExact = false
		m.intentionInput.Blur()
		m.rateInput.Blur()
		return m, nil
	}
	var cmd tea.Cmd
	m.intentionInput, cmd = m.intentionInput.Update(msg)
	return m, cmd
}

func (m otcFXModel) updateRate(msg tea.KeyMsg) (otcFXModel, tea.Cmd) {
	if m.rateExact {
		switch {
		case key.Matches(msg, keys.Enter):
			val := strings.TrimSpace(m.rateInput.Value())
			if val == "" {
				m.err = fmt.Errorf("enter a rate or press +/- for bps mode")
				return m, nil
			}
			var rate float64
			_, err := fmt.Sscanf(val, "%f", &rate)
			if err != nil || rate <= 0 {
				m.err = fmt.Errorf("invalid rate")
				return m, nil
			}
			m.exactRate = rate
			m.err = nil
			m.step = fxStepConfirm
			m.rateInput.Blur()
			return m, nil
		case msg.String() == "+" || msg.String() == "-":
			m.rateExact = false
			m.exactRate = 0
			m.rateInput.Blur()
			if msg.String() == "+" {
				m.spreadBps++
			} else if m.spreadBps > 0 {
				m.spreadBps--
			}
			return m, nil
		}
		var cmd tea.Cmd
		m.rateInput, cmd = m.rateInput.Update(msg)
		return m, cmd
	}

	// BPS mode
	switch {
	case msg.String() == "+":
		m.spreadBps++
	case msg.String() == "-":
		if m.spreadBps > 0 {
			m.spreadBps--
		}
	case key.Matches(msg, keys.Enter):
		m.err = nil
		m.step = fxStepConfirm
	default:
		ch := msg.String()
		if len(ch) == 1 && ch[0] >= '0' && ch[0] <= '9' {
			m.rateExact = true
			m.rateInput.SetValue(ch)
			m.rateInput.Focus()
			m.rateInput.SetCursor(1)
		}
	}
	return m, nil
}

func (m otcFXModel) updateConfirm(msg tea.KeyMsg, c *client.Client) (otcFXModel, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter":
		return m, m.executeTrade(c)
	case "n", "N":
		m.cancelled = true
	}
	return m, nil
}

func (m *otcFXModel) executeTrade(c *client.Client) tea.Cmd {
	gives := m.customerGives()
	gets := m.customerGets()
	srcCur := m.sourceCur()
	dstCur := m.destCur()

	rate := m.effectiveRate()
	mid := m.midRate()
	bps := m.spreadFromRate(rate)

	desc := fmt.Sprintf("OTC FX: %s %s → %s %s @ %.4f (mid %.4f %+.0fbps)",
		ledger.FormatAmount(gives, srcCur), srcCur,
		ledger.FormatAmount(gets, dstCur), dstCur,
		rate, mid, bps)

	txn := &ledger.Transaction{
		Description: desc,
		Entries: []ledger.Entry{
			{AccountID: m.srcAcctID, Amount: gives, Currency: srcCur},
			{AccountID: "~fx", Amount: -gives, Currency: srcCur},
			{AccountID: "~fx", Amount: gets, Currency: dstCur},
			{AccountID: m.destAcctID, Amount: -gets, Currency: dstCur},
		},
	}

	return func() tea.Msg {
		created, err := c.CreateTransaction(context.Background(), txn)
		return otcFXTxnCreatedMsg{txn: created, err: err}
	}
}

// --- Views ---

func (m *otcFXModel) view() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("OTC Foreign Exchange"))
	b.WriteString("\n")

	if m.srcCurrency != "" && m.destCurrency != "" {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  %s → %s   Mid: %.4f", m.srcCurrency, m.destCurrency, m.midRate())))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	switch m.step {
	case fxStepLanding:
		m.viewLanding(&b)
	case fxStepSrcAcct:
		m.viewSrcAcct(&b)
	case fxStepDestAcct:
		m.viewDestAcct(&b)
	case fxStepIntention:
		m.viewIntention(&b)
	case fxStepRate:
		m.viewRate(&b)
	case fxStepConfirm:
		m.viewConfirm(&b)
	}

	if m.err != nil {
		b.WriteString("\n" + errorStyle.Render("  Error: "+m.err.Error()) + "\n")
	}

	if m.step > fxStepLanding {
		steps := []string{"Source", "Dest", "Amount", "Rate", "Confirm"}
		idx := int(m.step) - 1
		b.WriteString("\n" + dimStyle.Render(fmt.Sprintf("  Step %d/%d: %s", idx+1, len(steps), steps[idx])))
		b.WriteString("\n" + dimStyle.Render("  ESC to go back"))
	}
	return b.String()
}

func (m *otcFXModel) viewLanding(b *strings.Builder) {
	b.WriteString("  Press " + selectedStyle.Render("f") + " to start a new FX deal\n\n")
	b.WriteString(dimStyle.Render("  The OTC FX desk lets you build and execute") + "\n")
	b.WriteString(dimStyle.Render("  cross-currency trades against the ledger.") + "\n")
}

func (m *otcFXModel) viewSrcAcct(b *strings.Builder) {
	b.WriteString("  Customer source account (sells from):\n\n")
	b.WriteString("  " + m.srcAcctInput.View() + "\n")
	m.viewAccountHints(b, "")
}

func (m *otcFXModel) viewDestAcct(b *strings.Builder) {
	b.WriteString(fmt.Sprintf("  Source: %s (%s)\n\n", m.srcAcctID, m.srcCurrency))
	b.WriteString("  Customer dest account (buys into):\n\n")
	b.WriteString("  " + m.destAcctInput.View() + "\n")
	m.viewAccountHints(b, m.srcCurrency)
}

func (m *otcFXModel) viewIntention(b *strings.Builder) {
	b.WriteString("  Enter amount and direction:\n\n")
	b.WriteString("  " + m.intentionInput.View() + "\n\n")

	b.WriteString(dimStyle.Render(fmt.Sprintf("  'give <amount>'  = customer sells %s (from %s)", m.srcCurrency, m.srcAcctID)) + "\n")
	b.WriteString(dimStyle.Render(fmt.Sprintf("  'get <amount>'   = customer buys %s (into %s)", m.destCurrency, m.destAcctID)) + "\n")
	b.WriteString(dimStyle.Render("  currency is optional, e.g. 'give 100' or 'give 100 "+m.srcCurrency+"'") + "\n")
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(fmt.Sprintf("  Mid rate: 1 %s = %.4f %s", m.srcCurrency, m.midRate(), m.destCurrency)) + "\n")
}

func (m *otcFXModel) viewRate(b *strings.Builder) {
	rate := m.effectiveRate()
	gives := m.customerGives()
	gets := m.customerGets()
	pnl := m.bankPnLGEL()
	srcCur := m.sourceCur()
	dstCur := m.destCur()

	b.WriteString("  Adjust rate:\n\n")

	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("Mid rate:   1 %s = %.4f %s\n", srcCur, m.midRate(), dstCur))
	if m.rateExact {
		summary.WriteString("Mode:       Exact rate\n")
	} else {
		summary.WriteString(fmt.Sprintf("Spread:     %d bps\n", m.spreadBps))
	}
	summary.WriteString(fmt.Sprintf("Deal rate:  1 %s = %.4f %s\n", srcCur, rate, dstCur))
	summary.WriteString("\n")
	summary.WriteString(fmt.Sprintf("Customer gives:  %s %s\n", ledger.FormatAmount(gives, srcCur), srcCur))
	summary.WriteString(fmt.Sprintf("Customer gets:   %s %s\n", ledger.FormatAmount(gets, dstCur), dstCur))
	summary.WriteString(fmt.Sprintf("Bank P&L:        %s GEL", ledger.FormatAmount(pnl, "GEL")))

	b.WriteString(boxStyle.Render(summary.String()))
	b.WriteString("\n\n")

	if m.rateExact {
		b.WriteString("  Rate: " + m.rateInput.View() + "\n")
		b.WriteString(dimStyle.Render("  +/- to switch back to bps mode") + "\n")
	} else {
		b.WriteString(dimStyle.Render("  +/- to adjust spread  |  type digits for exact rate  |  enter to confirm") + "\n")
	}
}

func (m *otcFXModel) viewConfirm(b *strings.Builder) {
	rate := m.effectiveRate()
	mid := m.midRate()
	gives := m.customerGives()
	gets := m.customerGets()
	pnl := m.bankPnLGEL()
	srcCur := m.sourceCur()
	dstCur := m.destCur()
	bps := m.spreadFromRate(rate)

	b.WriteString("  Deal summary:\n\n")

	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("Pair:            %s → %s\n", srcCur, dstCur))
	summary.WriteString(fmt.Sprintf("Deal rate:       %.4f (mid %.4f %+.0f bps)\n", rate, mid, bps))
	summary.WriteString(fmt.Sprintf("Customer gives:  %s %s\n", ledger.FormatAmount(gives, srcCur), srcCur))
	summary.WriteString(fmt.Sprintf("Customer gets:   %s %s\n", ledger.FormatAmount(gets, dstCur), dstCur))
	summary.WriteString(fmt.Sprintf("Bank P&L:        %s GEL\n", ledger.FormatAmount(pnl, "GEL")))
	summary.WriteString("\n")
	summary.WriteString(fmt.Sprintf("Source acct:     %s (%s)\n", m.srcAcctID, srcCur))
	summary.WriteString(fmt.Sprintf("Dest acct:       %s (%s)", m.destAcctID, dstCur))

	b.WriteString(boxStyle.Render(summary.String()))
	b.WriteString("\n\n")

	b.WriteString(selectedStyle.Render("  Execute trade? (y/n)") + "\n")
}

// viewAccountHints shows non-system accounts, excluding those with excludeCurrency.
func (m *otcFXModel) viewAccountHints(b *strings.Builder, excludeCurrency string) {
	var hints []string
	for _, a := range m.accounts {
		if a.IsSystem || a.Currency == "*" {
			continue
		}
		if excludeCurrency != "" && a.Currency == excludeCurrency {
			continue
		}
		name := a.Name
		if len(name) > 20 {
			name = name[:20] + ".."
		}
		hints = append(hints, fmt.Sprintf("%-12s %-22s %s", a.ID, name, a.Currency))
	}
	if len(hints) > 0 {
		b.WriteString("\n" + dimStyle.Render("  Accounts:") + "\n")
		for _, h := range hints {
			b.WriteString(dimStyle.Render("    "+h) + "\n")
		}
	}
}
