package tui

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("99")).
			MarginBottom(1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	activeTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("99")).
			Background(lipgloss.Color("236")).
			Padding(0, 2)

	inactiveTabStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245")).
				Padding(0, 2)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginTop(1)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("82"))

	debitStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("82"))

	creditStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	labelStyle = lipgloss.NewStyle().
			Bold(true).
			Width(16)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("99")).
			Bold(true)

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("252")).
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("240"))

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(1, 2)

	hintBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("99")).
			Padding(1, 2)
)
