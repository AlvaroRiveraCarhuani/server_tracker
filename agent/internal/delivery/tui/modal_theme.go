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

	escBadge := lipgloss.NewStyle().Foreground(ColorSubtext0).Render("esc")
	headerLeft := lipgloss.NewStyle().Bold(true).Foreground(ColorLavender).Render("Seleccionar Tema y Tipografía")
	spLen := max(1, innerW-lipgloss.Width(headerLeft)-lipgloss.Width(escBadge))
	header := headerLeft + strings.Repeat(" ", spLen) + escBadge

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
			swatches = append(swatches, lipgloss.NewStyle().Foreground(lipgloss.Color(hex)).Render("■"))
		}
		swatchStr := strings.Join(swatches, "")

		namePart := fmt.Sprintf("%s%-18s %s", badge, th.Name, swatchStr)
		descPart := truncate(th.Description, max(15, innerW-lipgloss.Width(namePart)-4))
		sp := max(1, innerW-lipgloss.Width(namePart)-lipgloss.Width(descPart)-2)
		rowText := fmt.Sprintf("  %s%s%s", namePart, strings.Repeat(" ", sp), descPart)

		if i == m.themeListCursor {
			rowStyled := lipgloss.NewStyle().
				Foreground(ColorPeach).
				Bold(true).
				Width(innerW).
				Render(rowText)
			lines = append(lines, rowStyled)
		} else {
			rowStyled := lipgloss.NewStyle().
				Foreground(ColorText).
				Width(innerW).
				Render(rowText)
			lines = append(lines, rowStyled)
		}
	}

	listBlock := strings.Join(lines, "\n")
	sep := lipgloss.NewStyle().Foreground(ColorSurface1).Render(strings.Repeat("─", innerW))

	// Toggles de configuración estética
	nerdStatus := lipgloss.NewStyle().Foreground(ColorRed).Render("[OFF - ASCII Seguro]")
	if m.themeConfig.NerdFonts {
		nerdStatus = lipgloss.NewStyle().Foreground(ColorGreen).Bold(true).Render("[ON - 󰄴 󰡨 ]")
	}

	borderStatus := lipgloss.NewStyle().Foreground(ColorLavender).Render(fmt.Sprintf("[%s]", strings.ToUpper(m.themeConfig.BorderStyle)))

	togglesLine := lipgloss.NewStyle().Foreground(ColorSubtext1).Render(
		fmt.Sprintf("  [f] Nerd Fonts: %s    [b] Bordes: %s", nerdStatus, borderStatus),
	)

	footer := lipgloss.NewStyle().Foreground(ColorSubtext0).Render(
		"Enter: Aplicar y Guardar   f: Alternar Iconos   b: Alternar Bordes   Esc: Cerrar",
	)

	body := fmt.Sprintf("%s\n\n%s\n\n%s\n%s\n\n%s", header, listBlock, sep, togglesLine, footer)
	return StyleModal.Width(modalWidth).Render(body)
}
