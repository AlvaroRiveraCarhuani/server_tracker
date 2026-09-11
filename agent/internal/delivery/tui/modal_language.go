package tui

import (
	"fmt"
	"strings"

	"github.com/alvaroriverac/server_tracker_agent/internal/i18n"
	"github.com/charmbracelet/lipgloss"
)

// viewFirstRunLanguageOverlay renderiza el overlay modal de primera ejecución para selección de idioma (C4).
func (m Model) viewFirstRunLanguageOverlay() string {
	modalWidth := 46
	if m.width > 20 && m.width-4 < modalWidth {
		modalWidth = m.width - 4
	}
	innerW := modalWidth - 6

	bgStyle := lipgloss.NewStyle().Background(ColorSurface0)
	escBadge := lipgloss.NewStyle().Foreground(ColorSubtext0).Background(ColorSurface0).Render("esc")
	titleText := i18n.T(m.language, "first_run.title")
	headerLeft := lipgloss.NewStyle().Bold(true).Foreground(ColorPeach).Background(ColorSurface0).Render(titleText)
	spLen := max(1, innerW-lipgloss.Width(headerLeft)-lipgloss.Width(escBadge))
	header := headerLeft + bgStyle.Render(strings.Repeat(" ", spLen)) + escBadge

	var lines []string
	subtitle := lipgloss.NewStyle().Foreground(ColorSubtext1).Background(ColorSurface0).Bold(true).Render(i18n.T(m.language, "first_run.subtitle"))
	lines = append(lines, subtitle)
	lines = append(lines, "")

	// Opción 0: Español
	opt0 := "español"
	if m.langOverlayCursor == 0 {
		cursorStyle := lipgloss.NewStyle().Foreground(ColorPeach).Background(ColorSurface0).Bold(true)
		lines = append(lines, cursorStyle.Render(" > "+opt0))
	} else {
		normalStyle := lipgloss.NewStyle().Foreground(ColorText).Background(ColorSurface0)
		lines = append(lines, normalStyle.Render("   "+opt0))
	}

	// Opción 1: English
	opt1 := "english"
	if m.langOverlayCursor == 1 {
		cursorStyle := lipgloss.NewStyle().Foreground(ColorPeach).Background(ColorSurface0).Bold(true)
		lines = append(lines, cursorStyle.Render(" > "+opt1))
	} else {
		normalStyle := lipgloss.NewStyle().Foreground(ColorText).Background(ColorSurface0)
		lines = append(lines, normalStyle.Render("   "+opt1))
	}

	lines = append(lines, "")
	sep := lipgloss.NewStyle().Foreground(ColorSurface1).Background(ColorSurface0).Render(strings.Repeat("─", innerW))
	lines = append(lines, sep)

	hintText := i18n.T(m.language, "first_run.hint")
	footer := lipgloss.NewStyle().Foreground(ColorSubtext0).Background(ColorSurface0).Render(hintText)
	lines = append(lines, footer)

	body := fmt.Sprintf("%s\n\n%s", header, strings.Join(lines, "\n"))
	return StyleModal.Width(modalWidth).Render(body)
}
