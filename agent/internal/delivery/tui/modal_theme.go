package tui

import (
	"fmt"
	"strings"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
	"github.com/charmbracelet/lipgloss"
)

func (m Model) viewThemeModal() string {
	modalWidth := 74
	if m.width > 20 && m.width-4 < modalWidth {
		modalWidth = m.width - 4
	}
	innerW := modalWidth - 6

	bgStyle := lipgloss.NewStyle().Background(ColorSurface0)
	escBadge := lipgloss.NewStyle().Foreground(ColorSubtext0).Background(ColorSurface0).Render("esc")
	headerLeft := lipgloss.NewStyle().Bold(true).Foreground(ColorLavender).Background(ColorSurface0).Render("Seleccionar Tema y Tipografía")
	spLen := max(1, innerW-lipgloss.Width(headerLeft)-3)
	header := headerLeft + bgStyle.Render(strings.Repeat(" ", spLen)) + escBadge

	var lines []string

	for i, th := range domain.AvailableThemes {
		isActive := (th.ID == m.themeConfig.ActiveTheme)
		badge := "  "
		if isActive {
			badge = "● "
		}

		// Mini swatch de colores
		var swatches []string
		for _, hex := range th.PreviewHex {
			swatches = append(swatches, lipgloss.NewStyle().Foreground(lipgloss.Color(hex)).Background(ColorSurface0).Render("■"))
		}
		swatchStr := strings.Join(swatches, "")

		textColor := ColorText
		if i == m.themeListCursor {
			textColor = ColorPeach
		}

		nameStyle := lipgloss.NewStyle().Foreground(textColor).Background(ColorSurface0)
		if i == m.themeListCursor {
			nameStyle = nameStyle.Bold(true)
		}
		nameFormatted := fmt.Sprintf("  %s%-18s ", badge, th.Name)
		badgeAndName := nameStyle.Render(nameFormatted)

		usedW := lipgloss.Width(nameFormatted) + len(th.PreviewHex)
		maxDescW := max(15, innerW-usedW-2)
		descText := truncate(th.Description, maxDescW)
		descW := lipgloss.Width(descText)

		spLen := max(1, innerW-usedW-descW)
		spStr := bgStyle.Render(strings.Repeat(" ", spLen))

		descStyle := lipgloss.NewStyle().Foreground(ColorSubtext1).Background(ColorSurface0)
		if i == m.themeListCursor {
			descStyle = descStyle.Foreground(ColorPeach).Bold(true)
		}
		descStr := descStyle.Render(descText)

		rowLine := badgeAndName + swatchStr + spStr + descStr
		if curW := lipgloss.Width(rowLine); curW < innerW {
			rowLine += bgStyle.Render(strings.Repeat(" ", innerW-curW))
		}
		lines = append(lines, rowLine)
	}

	listBlock := strings.Join(lines, "\n")
	sep := lipgloss.NewStyle().Foreground(ColorSurface1).Background(ColorSurface0).Render(strings.Repeat("─", innerW))

	// Toggles de configuración estética
	nerdColor := ColorRed
	nerdText := "[OFF - ASCII Seguro]"
	if m.themeConfig.NerdFonts {
		nerdColor = ColorGreen
		nerdText = "[ON - 󰄴 󰡨 ]"
	}

	nerdLabel := lipgloss.NewStyle().Foreground(ColorSubtext1).Background(ColorSurface0).Render("  [f] Nerd Fonts: ")
	nerdVal := lipgloss.NewStyle().Foreground(nerdColor).Background(ColorSurface0).Bold(true).Render(nerdText)
	midSp := bgStyle.Render("    ")
	borderLabel := lipgloss.NewStyle().Foreground(ColorSubtext1).Background(ColorSurface0).Render("[b] Bordes: ")
	borderVal := lipgloss.NewStyle().Foreground(ColorLavender).Background(ColorSurface0).Render(fmt.Sprintf("[%s]", strings.ToUpper(m.themeConfig.BorderStyle)))

	togglesLine := nerdLabel + nerdVal + midSp + borderLabel + borderVal
	if tw := lipgloss.Width(togglesLine); tw < innerW {
		togglesLine += bgStyle.Render(strings.Repeat(" ", innerW-tw))
	}

	footer := lipgloss.NewStyle().Foreground(ColorSubtext0).Background(ColorSurface0).Width(innerW).Render(
		"Enter: Aplicar y Guardar   f: Alternar Iconos   b: Alternar Bordes   Esc: Cerrar",
	)

	body := fmt.Sprintf("%s\n\n%s\n\n%s\n%s\n\n%s", header, listBlock, sep, togglesLine, footer)
	return StyleModal.Width(modalWidth).Render(body)
}
