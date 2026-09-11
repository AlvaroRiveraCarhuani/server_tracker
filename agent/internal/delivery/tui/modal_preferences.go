package tui

import (
	"fmt"
	"strings"

	"github.com/alvaroriverac/server_tracker_agent/internal/i18n"
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
	titleText := i18n.T(m.language, "preferences.title")
	headerLeft := lipgloss.NewStyle().Bold(true).Foreground(ColorPeach).Background(ColorSurface0).Render(titleText)
	spLen := max(1, innerW-lipgloss.Width(headerLeft)-lipgloss.Width(escBadge))
	header := headerLeft + bgStyle.Render(strings.Repeat(" ", spLen)) + escBadge

	var lines []string

	// Fila 0: Temas y estilos
	row0Label := i18n.T(m.language, "preferences.theme")
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
	policyKey := "preferences.policy_info"
	if policy == "prudente" {
		policyKey = "preferences.policy_prud"
	}
	policyTranslated := i18n.T(m.language, policyKey)
	row1Label := i18n.T(m.language, "preferences.banner_policy", map[string]interface{}{"policy": policyTranslated})
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

	// Fila 2: Idioma / Language (Ola 8)
	langDisplay := "español"
	if m.language == i18n.LangEN {
		langDisplay = "english"
	}
	row2Label := i18n.T(m.language, "preferences.language", map[string]interface{}{"lang": langDisplay})
	row2Color := ColorText
	if m.preferencesCursor == 2 {
		row2Color = ColorPeach
	}
	row2Style := lipgloss.NewStyle().Foreground(row2Color).Background(ColorSurface0)
	if m.preferencesCursor == 2 {
		row2Style = row2Style.Bold(true)
	}
	row2Left := "  " + row2Label
	if m.preferencesCursor == 2 {
		row2Left = "> " + row2Label
	}
	sp2 := max(1, innerW-lipgloss.Width(row2Left)-lipgloss.Width(enterBadge))
	badge2Style := lipgloss.NewStyle().Foreground(ColorSubtext1).Background(ColorSurface0)
	if m.preferencesCursor == 2 {
		badge2Style = lipgloss.NewStyle().Foreground(ColorPeach).Background(ColorSurface0).Bold(true)
	}
	row2Str := row2Style.Render(row2Left) + bgStyle.Render(strings.Repeat(" ", sp2)) + badge2Style.Render(enterBadge)
	lines = append(lines, row2Str)

	sep := lipgloss.NewStyle().Foreground(ColorSurface1).Background(ColorSurface0).Render(strings.Repeat("─", innerW))
	footer := lipgloss.NewStyle().Foreground(ColorSubtext0).Background(ColorSurface0).Render(i18n.T(m.language, "preferences.footer"))

	body := fmt.Sprintf("%s\n\n%s\n\n%s\n%s", header, strings.Join(lines, "\n"), sep, footer)
	return StyleModal.Width(modalWidth).Render(body)
}
