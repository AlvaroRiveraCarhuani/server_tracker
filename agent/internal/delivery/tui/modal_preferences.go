package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// viewPreferencesModal renderiza el modal de preferencias del operador (renombrado de 't').
func (m Model) viewPreferencesModal() string {
	modalWidth := 56
	if m.width > 20 && m.width-4 < modalWidth {
		modalWidth = m.width - 4
	}
	innerW := modalWidth - 6

	bgStyle := lipgloss.NewStyle().Background(ColorSurface0)
	escBadge := lipgloss.NewStyle().Foreground(ColorSubtext0).Background(ColorSurface0).Render("esc")
	headerLeft := lipgloss.NewStyle().Bold(true).Foreground(ColorPeach).Background(ColorSurface0).Render("preferencias")
	spLen := max(1, innerW-lipgloss.Width(headerLeft)-3)
	header := headerLeft + bgStyle.Render(strings.Repeat(" ", spLen)) + escBadge

	var lines []string

	// Fila 0: Temas y estilos
	row0Label := "temas y estilos"
	enterBadge := "[Enter]"
	row0Color := ColorText
	if m.preferencesCursor == 0 {
		row0Color = ColorPeach
	}
	row0Style := lipgloss.NewStyle().Foreground(row0Color).Background(ColorSurface0)
	if m.preferencesCursor == 0 {
		row0Style = row0Style.Bold(true)
	}
	row0Left := "  " + row0Label
	if m.preferencesCursor == 0 {
		row0Left = "> " + row0Label
	}
	sp0 := max(1, innerW-lipgloss.Width(row0Left)-lipgloss.Width(enterBadge))
	badge0Style := lipgloss.NewStyle().Foreground(ColorSubtext1).Background(ColorSurface0)
	if m.preferencesCursor == 0 {
		badge0Style = lipgloss.NewStyle().Foreground(ColorPeach).Background(ColorSurface0).Bold(true)
	}
	row0Str := row0Style.Render(row0Left) + bgStyle.Render(strings.Repeat(" ", sp0)) + badge0Style.Render(enterBadge)
	lines = append(lines, row0Str)

	// Fila 1: Origen en banner
	policy := m.themeConfig.IncidentBannerPolicy
	if policy == "" {
		policy = "informativo"
	}
	row1Label := fmt.Sprintf("origen en banner: %s", policy)
	row1Color := ColorText
	if m.preferencesCursor == 1 {
		row1Color = ColorPeach
	}
	row1Style := lipgloss.NewStyle().Foreground(row1Color).Background(ColorSurface0)
	if m.preferencesCursor == 1 {
		row1Style = row1Style.Bold(true)
	}
	row1Left := "  " + row1Label
	if m.preferencesCursor == 1 {
		row1Left = "> " + row1Label
	}
	sp1 := max(1, innerW-lipgloss.Width(row1Left)-lipgloss.Width(enterBadge))
	badge1Style := lipgloss.NewStyle().Foreground(ColorSubtext1).Background(ColorSurface0)
	if m.preferencesCursor == 1 {
		badge1Style = lipgloss.NewStyle().Foreground(ColorPeach).Background(ColorSurface0).Bold(true)
	}
	row1Str := row1Style.Render(row1Left) + bgStyle.Render(strings.Repeat(" ", sp1)) + badge1Style.Render(enterBadge)
	lines = append(lines, row1Str)

	sep := lipgloss.NewStyle().Foreground(ColorSurface1).Background(ColorSurface0).Render(strings.Repeat("─", innerW))
	footer := lipgloss.NewStyle().Foreground(ColorSubtext0).Background(ColorSurface0).Render("enter: cambiar · esc: volver")

	body := fmt.Sprintf("%s\n\n%s\n\n%s\n%s", header, strings.Join(lines, "\n"), sep, footer)
	return StyleModal.Width(modalWidth).Render(body)
}
