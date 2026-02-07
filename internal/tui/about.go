package tui

import (
	"fmt"
	"math/rand"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var aboutUsers = []string{"Michael Bolton", "Samir Nagheenanajar", "Peter Gibbons"}

type aboutPage int

const (
	aboutMenu aboutPage = iota
	aboutProducts
	aboutAboutUs
	aboutSecurity
	aboutProgrammable
	aboutContact
	aboutLegal
	aboutHiring
)

var (
	bbsGreen = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	bbsBold  = lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Bold(true)
	bbsDim   = lipgloss.NewStyle().Foreground(lipgloss.Color("34"))
	bbsBar   = lipgloss.NewStyle().Background(lipgloss.Color("82")).Foreground(lipgloss.Color("0")).Bold(true)
)

const paveBankBanner = `██████   █████  ██    ██ ███████     ██████   █████  ███    ██ ██   ██
██   ██ ██   ██ ██    ██ ██          ██   ██ ██   ██ ████   ██ ██  ██
██████  ███████ ██    ██ █████       ██████  ███████ ██ ██  ██ █████
██      ██   ██  ██  ██  ██          ██   ██ ██   ██ ██  ██ ██ ██  ██
██      ██   ██   ████   ███████     ██████  ██   ██ ██   ████ ██   ██`

type aboutModel struct {
	page   aboutPage
	width  int
	height int
	user   string
}

func newAboutModel() aboutModel {
	return aboutModel{user: aboutUsers[rand.Intn(len(aboutUsers))]}
}

func (m aboutModel) update(msg tea.Msg) (aboutModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.page != aboutMenu {
			if msg.String() == "esc" {
				m.page = aboutMenu
				return m, nil
			}
			return m, nil
		}

		switch msg.String() {
		case "1":
			m.page = aboutProducts
		case "2":
			m.page = aboutAboutUs
		case "3":
			m.page = aboutSecurity
		case "4":
			m.page = aboutProgrammable
		case "5":
			m.page = aboutContact
		case "6":
			m.page = aboutLegal
		case "7":
			m.page = aboutHiring
		}
	}
	return m, nil
}

func (m *aboutModel) view() string {
	switch m.page {
	case aboutProducts:
		return m.viewContentPage("Products & Services", contentProducts)
	case aboutAboutUs:
		return m.viewContentPage("About Pave Bank", contentAbout)
	case aboutSecurity:
		return m.viewContentPage("Security Information", contentSecurity)
	case aboutProgrammable:
		return m.viewContentPage("Programmable Banking", contentProgrammable)
	case aboutContact:
		return m.viewContentPage("Contact Us", contentContact)
	case aboutLegal:
		return m.viewContentPage("Legal Information", contentLegal)
	case aboutHiring:
		return m.viewContentPage("WE'RE HIRING", contentHiring)
	default:
		return m.viewMenu()
	}
}

func (m *aboutModel) separator() string {
	w := m.width - 4
	if w < 40 {
		w = 40
	}
	if w > 76 {
		w = 76
	}
	return bbsDim.Render("  " + strings.Repeat("═", w))
}

func (m *aboutModel) thinSeparator() string {
	w := m.width - 4
	if w < 40 {
		w = 40
	}
	if w > 76 {
		w = 76
	}
	return bbsDim.Render("  " + strings.Repeat("─", w))
}

func (m *aboutModel) viewMenu() string {
	var b strings.Builder

	// Banner
	for _, line := range strings.Split(paveBankBanner, "\n") {
		b.WriteString(bbsBold.Render(line))
		b.WriteString("\n")
	}

	b.WriteString(m.separator() + "\n")
	b.WriteString(bbsGreen.Render(center("MAINFRAME v1.0", 70)) + "\n")
	b.WriteString(bbsBold.Render(center("Licensed Bank of Georgia #305", 70)) + "\n")
	b.WriteString(m.separator() + "\n")
	b.WriteString("\n")

	// Main Menu label
	b.WriteString(bbsBar.Render("  MAIN MENU  ") + "  " + bbsDim.Render("Press Tab to change page") + "\n")
	b.WriteString("\n")
	b.WriteString(bbsGreen.Render("Welcome to PAVE BANK MAINFRAME!") + "\n")
	b.WriteString(m.thinSeparator() + "\n")
	b.WriteString("\n")

	b.WriteString(bbsBold.Render(center("L I M I T L E S S   B A N K I N G", 70)) + "\n")
	b.WriteString("\n")
	b.WriteString(bbsGreen.Render("  Empowering businesses with 24/7 global, secure, multi-asset banking.") + "\n")
	b.WriteString("\n")
	b.WriteString(m.thinSeparator() + "\n")
	b.WriteString("\n")

	// Menu items
	items := []string{
		"Products & Services",
		"About Pave Bank",
		"Security Information",
		"Programmable Banking",
		"Contact Us",
		"Legal Information",
		"WE'RE HIRING",
	}
	for i, item := range items {
		label := fmt.Sprintf("  [%d] %s", i+1, item)
		if i == 6 {
			b.WriteString(bbsBold.Render(label) + "\n")
		} else {
			b.WriteString(bbsGreen.Render(label) + "\n")
		}
	}

	body := b.String()

	// Horizontal centering: content block is ≤78 chars wide.
	contentW := m.width - 4
	if contentW > 76 {
		contentW = 76
	}
	contentW += 2 // "  " prefix on separators
	leftPad := ""
	if m.width > contentW {
		leftPad = strings.Repeat(" ", (m.width-contentW)/2)
	}

	lines := strings.Split(body, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = leftPad + line
		}
	}
	body = strings.Join(lines, "\n")

	// Status bar (full width, not centered)
	left := " [1-7] Select "
	right := " User: " + m.user + " "
	gap := m.width - len(left) - len(right)
	if gap < 2 {
		gap = 2
	}
	statusBar := bbsBar.Render(left + strings.Repeat(" ", gap) + right)

	// Vertical centering: pad body lines above, status bar pinned below
	bodyLines := strings.Count(body, "\n") + 1
	totalLines := bodyLines + 2 // +2 for blank line + status bar
	topPad := 0
	if m.height > totalLines {
		topPad = (m.height - totalLines) / 2
	}

	return strings.Repeat("\n", topPad) + body + "\n\n" + statusBar
}

func (m *aboutModel) viewContentPage(title, content string) string {
	var b strings.Builder

	b.WriteString(m.separator() + "\n")
	b.WriteString(bbsBold.Render(center(title, 70)) + "\n")
	b.WriteString(m.separator() + "\n")
	b.WriteString("\n")

	for _, line := range strings.Split(content, "\n") {
		b.WriteString(bbsGreen.Render(line) + "\n")
	}

	b.WriteString("\n")
	b.WriteString(bbsDim.Render("  Press ESC to return to main menu") + "\n")

	return b.String()
}

func center(s string, width int) string {
	pad := (width - len(s)) / 2
	if pad < 0 {
		pad = 0
	}
	return strings.Repeat(" ", pad) + s
}

const contentProducts = `  Global Banking
  ─────────────
  Connect to international financial networks. Access your accounts
  from anywhere in the world, at any time. Our infrastructure operates
  24/7 with no downtime windows.

  Multi-Asset & Multi-Currency
  ────────────────────────────
  Manage multiple currencies on a single unified platform. Hold, send,
  and receive GEL, USD, EUR, GBP, CHF, JPY, and more from one account.

  Fully Regulated Commercial Bank
  ────────────────────────────────
  Your deposits are protected under Georgian law. We maintain full
  reserves and publish transparent financial reports.

  OTC Foreign Exchange
  ────────────────────
  Execute currency trades directly with the bank at competitive rates.
  Real-time pricing with configurable spread.`

const contentAbout = `  Pave Bank is a licensed commercial bank regulated by the National
  Bank of Georgia under License Number 305.

  We provide global, multi-asset banking services to businesses and
  individuals, operating 24/7 with modern technology infrastructure.

  Our mission is limitless banking — removing the barriers of
  geography, time zones, and legacy systems that hold back modern
  commerce.

  Founded in Tbilisi, Georgia, we serve clients worldwide through
  our digital-first platform.`

const contentSecurity = `  Deposit Insurance
  ─────────────────
  All eligible deposits are insured under the Georgian Deposit
  Insurance System, as mandated by Georgian banking law.

  Transparent Reporting
  ─────────────────────
  We publish regular financial reports and maintain full reserve
  disclosure. Our balance sheet is available for inspection.

  Security Protocols
  ──────────────────
  Comprehensive security covering:
    - Technology infrastructure and encryption
    - Operational security procedures
    - Risk management framework
    - Governance and compliance structures`

const contentProgrammable = `  Build on Top of Your Money
  ──────────────────────────
  Pave Bank goes beyond traditional banking APIs. Our programmable
  banking platform lets you automate and customize banking operations
  through code.

  Automate Workflows
  ──────────────────
  Set up automated transfers, reconciliation, and reporting. Integrate
  banking operations directly into your business logic.

  Developer-First
  ────────────────
  Full API access with modern tooling. Build custom financial products
  and services on top of our regulated banking infrastructure.`

const contentContact = `  Pave Bank
  Tbilisi, Georgia

  Online Banking:    app.pavebank.com
  Website:           www.pavebank.com
  Careers:           jobs.ashbyhq.com/pavebank`

const contentLegal = `  Pave Bank is licensed and regulated by the National Bank of
  Georgia under License Number 305.

  All banking services are provided in accordance with Georgian
  banking law and applicable international regulations.

  Deposits are protected under the Georgian Deposit Insurance
  System.

  Privacy policy and full legal documentation are available at
  www.pavebank.com.`

const contentHiring = `  ╔══════════════════════════════════════════════════════════╗
  ║                                                        ║
  ║          W E ' R E   H I R I N G !                     ║
  ║                                                        ║
  ║   Join us in building the future of banking.           ║
  ║                                                        ║
  ║   We're looking for engineers, designers, and          ║
  ║   banking professionals who want to rethink            ║
  ║   financial infrastructure from the ground up.         ║
  ║                                                        ║
  ║   View open positions:                                 ║
  ║                                                        ║
  ║     https://jobs.ashbyhq.com/pavebank                  ║
  ║                                                        ║
  ╚══════════════════════════════════════════════════════════╝`
