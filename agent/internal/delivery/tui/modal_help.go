package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) viewHelp() string {
	modalWidth := 72
	if m.width > 20 && m.width-4 < modalWidth {
		modalWidth = m.width - 4
	}
	innerW := modalWidth - 6

	bgStyle := lipgloss.NewStyle().Background(ColorSurface0)
	escBadge := lipgloss.NewStyle().Foreground(ColorSubtext0).Background(ColorSurface0).Render("esc")
	headerLeft := lipgloss.NewStyle().Bold(true).Foreground(ColorPeach).Background(ColorSurface0).Render("Atajos de teclado")
	spLen := max(1, innerW-lipgloss.Width(headerLeft)-3)
	header := headerLeft + bgStyle.Render(strings.Repeat(" ", spLen)) + escBadge

	colW := (innerW - 4) / 2
	sectionTitle := lipgloss.NewStyle().Bold(true).Foreground(ColorLavender).Background(ColorSurface0)
	keyStyle := lipgloss.NewStyle().Foreground(ColorPeach).Background(ColorSurface0).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(ColorText).Background(ColorSurface0)

	renderHelpRow := func(key, desc string) string {
		kStr := keyStyle.Render(fmt.Sprintf("  %-11s", key))
		descW := max(10, colW-13)
		dStr := descStyle.Render(fmt.Sprintf("%-*s", descW, desc))
		return kStr + dStr
	}

	renderSectionHeader := func(title string) string {
		return sectionTitle.Render(fmt.Sprintf("%-*s", colW, title))
	}

	// Columna Izquierda: Navegación & Modelos
	var leftLines []string
	leftLines = append(leftLines, renderSectionHeader("Navegación"))
	leftLines = append(leftLines, renderHelpRow("↑ / k", "Subir"))
	leftLines = append(leftLines, renderHelpRow("↓ / j", "Bajar"))
	leftLines = append(leftLines, renderHelpRow("p", "Fijar / Desanclar"))
	leftLines = append(leftLines, renderHelpRow("P", "Limpiar fijados"))
	leftLines = append(leftLines, renderHelpRow("Enter / l", "Ver logs"))
	leftLines = append(leftLines, renderHelpRow("/", "Buscar / Filtrar"))
	leftLines = append(leftLines, bgStyle.Render(strings.Repeat(" ", colW)))
	leftLines = append(leftLines, renderSectionHeader("Modelos de IA"))
	leftLines = append(leftLines, renderHelpRow("c", "Elegir modelo"))
	leftLines = append(leftLines, renderHelpRow("Ctrl+A", "Configurar API Key"))

	// Columna Derecha: Acciones & General
	var rightLines []string
	rightLines = append(rightLines, renderSectionHeader("Acciones"))
	rightLines = append(rightLines, renderHelpRow("r", "Reiniciar"))
	rightLines = append(rightLines, renderHelpRow("s", "Detener"))
	rightLines = append(rightLines, renderHelpRow("x", "Aislar de red"))
	rightLines = append(rightLines, renderHelpRow("e", "Abrir terminal"))
	rightLines = append(rightLines, bgStyle.Render(strings.Repeat(" ", colW)))
	rightLines = append(rightLines, renderSectionHeader("General"))
	rightLines = append(rightLines, renderHelpRow("t", "Temas y estilos"))
	rightLines = append(rightLines, renderHelpRow("?", "Ver esta ayuda"))
	rightLines = append(rightLines, renderHelpRow("q / Esc", "Cerrar / Salir"))
	rightLines = append(rightLines, bgStyle.Render(strings.Repeat(" ", colW)))
	rightLines = append(rightLines, bgStyle.Render(strings.Repeat(" ", colW)))

	leftBlock := bgStyle.Width(colW).Render(strings.Join(leftLines, "\n"))
	rightBlock := bgStyle.Width(colW).Render(strings.Join(rightLines, "\n"))
	sep := bgStyle.Render("    ")

	columnsRow := lipgloss.JoinHorizontal(lipgloss.Top, leftBlock, sep, rightBlock)
	centeredColumns := bgStyle.Width(innerW).Render(columnsRow)

	body := fmt.Sprintf("%s\n\n%s", header, centeredColumns)
	return StyleModal.Width(modalWidth).Render(body)
}
