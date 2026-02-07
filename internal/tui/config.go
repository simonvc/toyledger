package tui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/simonvc/miniledger/internal/client"
	"github.com/simonvc/miniledger/internal/ledger"
)

type settingsLoadedMsg struct {
	entries  []configRow
	err      error
}

type settingUpdatedMsg struct {
	err error
}

type configFlashClearMsg struct{}

type configRow struct {
	code     int
	name     string
	category ledger.Category
	settings ledger.CodeSettings
}

type configCol int

const (
	colBlockInverted configCol = iota
	colEntryDir
)

type configModel struct {
	rows     []configRow
	cursor   int
	col      configCol
	loading  bool
	err      error
	width    int
	height   int
	flashRow int // row index to flash, -1 for none
}

// validDirection checks if a direction is compatible with block-inverted for a category.
// Debit-normal (assets/expenses): CREDIT_ONLY is invalid (would force negative balance).
// Credit-normal (liabilities/equity/revenue): DEBIT_ONLY is invalid (would force positive balance).
func validDirection(cat ledger.Category, dir ledger.EntryDirection) bool {
	if dir == ledger.DirectionBoth {
		return true
	}
	isDebitNormal := cat == ledger.CategoryAssets || cat == ledger.CategoryExpenses
	if isDebitNormal && dir == ledger.DirectionCreditOnly {
		return false
	}
	if !isDebitNormal && dir == ledger.DirectionDebitOnly {
		return false
	}
	return true
}

// correctedDirection returns the safe default when the current direction is invalid.
func correctedDirection(cat ledger.Category) ledger.EntryDirection {
	isDebitNormal := cat == ledger.CategoryAssets || cat == ledger.CategoryExpenses
	if isDebitNormal {
		return ledger.DirectionDebitOnly
	}
	return ledger.DirectionCreditOnly
}

func (m *configModel) init(c *client.Client) tea.Cmd {
	m.loading = true
	return func() tea.Msg {
		// Load chart of accounts (both predefined and system)
		chart := ledger.AllChartEntries()

		// Load actual accounts to pick up codes not in the static chart
		accounts, acctErr := c.ListAccounts(context.Background(), "", nil)

		// Load all settings
		settings, err := c.ListSettings(context.Background())
		if err != nil {
			return settingsLoadedMsg{err: err}
		}

		// Build a map of code -> CodeSettings
		settingsMap := map[int]ledger.CodeSettings{}
		for _, s := range settings {
			cs, ok := settingsMap[s.Code]
			if !ok {
				cs = ledger.DefaultCodeSettings(s.Code)
			}
			switch s.Setting {
			case ledger.SettingBlockInverted:
				cs.BlockInverted = s.Value == "1"
			case ledger.SettingEntryDirection:
				cs.EntryDirection = ledger.EntryDirection(s.Value)
			}
			settingsMap[s.Code] = cs
		}

		// Build rows from chart entries, keyed by code
		codeSet := map[int]bool{}
		var rows []configRow
		for _, ce := range chart {
			cs, ok := settingsMap[ce.Code]
			if !ok {
				cs = ledger.DefaultCodeSettings(ce.Code)
			}
			rows = append(rows, configRow{
				code:     ce.Code,
				name:     ce.Name,
				category: ce.Category,
				settings: cs,
			})
			codeSet[ce.Code] = true
		}

		// Merge any account codes not already in the chart
		if acctErr == nil {
			for _, a := range accounts {
				if codeSet[a.Code] {
					continue
				}
				codeSet[a.Code] = true
				cs, ok := settingsMap[a.Code]
				if !ok {
					cs = ledger.DefaultCodeSettings(a.Code)
				}
				rows = append(rows, configRow{
					code:     a.Code,
					name:     a.Name,
					category: a.Category,
					settings: cs,
				})
			}
		}

		sort.Slice(rows, func(i, j int) bool { return rows[i].code < rows[j].code })

		return settingsLoadedMsg{entries: rows}
	}
}

func (m configModel) update(msg tea.Msg, c *client.Client) (configModel, tea.Cmd) {
	switch msg := msg.(type) {
	case settingsLoadedMsg:
		m.loading = false
		m.rows = msg.entries
		m.err = msg.err
		m.flashRow = -1

	case settingUpdatedMsg:
		if msg.err != nil {
			m.err = msg.err
		}

	case configFlashClearMsg:
		m.flashRow = -1
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, keys.Down):
			if m.cursor < len(m.rows)-1 {
				m.cursor++
			}
		case msg.String() == "left" || msg.String() == "h":
			if m.col > colBlockInverted {
				m.col--
			}
		case msg.String() == "right" || msg.String() == "l":
			if m.col < colEntryDir {
				m.col++
			}
		case msg.String() == " " || key.Matches(msg, keys.Enter):
			if m.cursor >= 0 && m.cursor < len(m.rows) {
				return m, m.toggleSetting(c)
			}
		}
	}
	return m, nil
}

func (m *configModel) toggleSetting(c *client.Client) tea.Cmd {
	row := &m.rows[m.cursor]
	code := row.code
	cursor := m.cursor

	switch m.col {
	case colBlockInverted:
		row.settings.BlockInverted = !row.settings.BlockInverted
		val := "0"
		if row.settings.BlockInverted {
			val = "1"
		}

		var cmds []tea.Cmd
		cmds = append(cmds, func() tea.Msg {
			err := c.UpsertSetting(context.Background(), code, ledger.SettingBlockInverted, val)
			return settingUpdatedMsg{err: err}
		})

		// Auto-correct direction if now invalid
		if row.settings.BlockInverted && !validDirection(row.category, row.settings.EntryDirection) {
			row.settings.EntryDirection = correctedDirection(row.category)
			m.flashRow = cursor
			newDir := row.settings.EntryDirection
			cmds = append(cmds, func() tea.Msg {
				err := c.UpsertSetting(context.Background(), code, ledger.SettingEntryDirection, string(newDir))
				return settingUpdatedMsg{err: err}
			})
			cmds = append(cmds, tea.Tick(800*time.Millisecond, func(time.Time) tea.Msg {
				return configFlashClearMsg{}
			}))
		}
		return tea.Batch(cmds...)

	case colEntryDir:
		// Cycle through valid directions only
		cycle := []ledger.EntryDirection{ledger.DirectionBoth, ledger.DirectionDebitOnly, ledger.DirectionCreditOnly}
		cur := row.settings.EntryDirection
		idx := 0
		for i, d := range cycle {
			if d == cur {
				idx = i
				break
			}
		}
		// Find next valid direction
		for step := 1; step <= len(cycle); step++ {
			next := cycle[(idx+step)%len(cycle)]
			if !row.settings.BlockInverted || validDirection(row.category, next) {
				row.settings.EntryDirection = next
				break
			}
		}
		newDir := row.settings.EntryDirection
		return func() tea.Msg {
			err := c.UpsertSetting(context.Background(), code, ledger.SettingEntryDirection, string(newDir))
			return settingUpdatedMsg{err: err}
		}
	}
	return nil
}

func (m *configModel) view() string {
	if m.loading {
		return "Loading settings..."
	}
	if m.err != nil {
		return errorStyle.Render("Error: " + m.err.Error())
	}
	if len(m.rows) == 0 {
		return dimStyle.Render("No chart of accounts entries found.")
	}

	var b strings.Builder

	b.WriteString(titleStyle.Render("Code Settings"))
	b.WriteString("\n")

	// Column widths
	codeW := 6
	nameW := 34
	normalW := 9
	blockW := 16

	// Header
	header := fmt.Sprintf("  %-*s%-*s%-*s%-*s%s",
		codeW, "CODE",
		nameW, "NAME",
		normalW, "NORMAL",
		blockW, "BLOCK_INVERTED",
		"ENTRY_DIR",
	)
	b.WriteString(headerStyle.Render(header))
	b.WriteString("\n")

	maxRows := m.height - 14
	if maxRows < 5 {
		maxRows = 5
	}

	start := 0
	if m.cursor >= maxRows {
		start = m.cursor - maxRows + 1
	}

	for i := start; i < len(m.rows) && i < start+maxRows; i++ {
		row := m.rows[i]
		name := row.name
		if len(name) > nameW-2 {
			name = name[:nameW-2] + ".."
		}
		normal := ledger.NormalBalance(row.category)

		blockRaw := "[ ]"
		if row.settings.BlockInverted {
			blockRaw = "[x]"
		}
		dirRaw := string(row.settings.EntryDirection)

		// Pad to column width first, then apply styling (ANSI codes break %-*s padding)
		blockCell := fmt.Sprintf("%-*s", blockW, blockRaw)
		dirCell := dirRaw

		if i == m.flashRow {
			dirCell = successStyle.Render(dirCell)
		} else if i == m.cursor {
			switch m.col {
			case colBlockInverted:
				blockCell = selectedStyle.Render(blockCell)
			case colEntryDir:
				dirCell = selectedStyle.Render(dirCell)
			}
		}

		line := fmt.Sprintf("  %-*d%-*s%-*s%s%s",
			codeW, row.code,
			nameW, name,
			normalW, normal,
			blockCell,
			dirCell,
		)

		if i == m.cursor {
			b.WriteString(selectedStyle.Render("> ") + line[2:])
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}

	b.WriteString(fmt.Sprintf("\n  %d codes", len(m.rows)))
	b.WriteString(dimStyle.Render("  |  arrows: navigate  space/enter: toggle  left/right: switch column"))

	// IFRS callout for selected row
	if m.cursor >= 0 && m.cursor < len(m.rows) {
		row := m.rows[m.cursor]
		normal := ledger.NormalBalance(row.category)
		var lines []string

		// Header: use chart entry description if available, otherwise category label
		entry := ledger.LookupChartEntry(row.code)
		if entry != nil {
			lines = append(lines,
				fmt.Sprintf("%s  IFRS %d — %s", headerStyle.Render(""), entry.Code, entry.Name),
				fmt.Sprintf("%s  %s", dimStyle.Render(""), entry.Description),
			)
		} else {
			lines = append(lines,
				fmt.Sprintf("%s  IFRS %d — %s", headerStyle.Render(""), row.code, row.name),
				fmt.Sprintf("%s  %s account (code %dxxx)", dimStyle.Render(""), ledger.CategoryLabel(row.category), row.code/1000),
			)
		}
		lines = append(lines, "")

		lines = append(lines, configSuggestion(row.code, row.category, normal)...)

		b.WriteString("\n\n")
		b.WriteString(boxStyle.Render(strings.Join(lines, "\n")))
	}

	return b.String()
}

// Per-code hint overrides. Only codes that deserve a specific callout go here.
type codeHint struct {
	block string
	dir   string
}

var codeHints = map[int]codeHint{
	1060: {dir: "DEBIT_ONLY recommended — regulatory reserves should only receive funds."},
	2020: {dir: "CREDIT_ONLY recommended — customer deposit accounts should only receive credits."},
}

// Category-level fallback hints, used when no code-specific hint exists.
var categoryHints = map[ledger.Category]codeHint{
	ledger.CategoryAssets: {
		block: "Prevents overdraw — asset balances should not go negative (credit-heavy).",
		dir:   "BOTH — assets normally accept debits (increases) and credits (decreases).",
	},
	ledger.CategoryLiabilities: {
		block: "Prevents liability accounts from flipping to a debit (asset-like) balance.",
		dir:   "BOTH — liabilities normally accept credits (increases) and debits (settlements).",
	},
	ledger.CategoryEquity: {
		block: "Prevents equity erosion below zero — protects against insolvency.",
		dir:   "BOTH — equity accepts credits (contributions/profit) and debits (distributions).",
	},
	ledger.CategoryRevenue: {
		block: "Prevents revenue reversal past zero — revenue should not go into debit.",
		dir:   "CREDIT_ONLY recommended — revenue accounts should only receive credits.",
	},
	ledger.CategoryExpenses: {
		block: "Prevents expense accounts from going negative (credit-heavy).",
		dir:   "DEBIT_ONLY recommended — expenses should only be debited.",
	},
}

func configSuggestion(code int, cat ledger.Category, normal string) []string {
	var lines []string

	lines = append(lines, headerStyle.Render("")+"  Suggested constraints:")

	fallback := categoryHints[cat]
	specific := codeHints[code]

	blockHint := fallback.block
	if specific.block != "" {
		blockHint = specific.block
	}
	dirHint := fallback.dir
	if specific.dir != "" {
		dirHint = specific.dir
	}

	lines = append(lines,
		fmt.Sprintf("  BLOCK_INVERTED  %s", dimStyle.Render(blockHint)),
		fmt.Sprintf("  ENTRY_DIR       %s", dimStyle.Render(dirHint)),
	)

	lines = append(lines, "",
		dimStyle.Render(fmt.Sprintf("  %s-normal account: positive balance means more %ss than %ss.",
			normal,
			strings.ToLower(normal),
			map[string]string{"Debit": "credit", "Credit": "debit"}[normal],
		)),
	)

	return lines
}
