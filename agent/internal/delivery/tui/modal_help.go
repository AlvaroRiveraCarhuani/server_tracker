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

	escBadge := lipgloss.NewStyle().Foreground(ColorSubtext0).Render("esc")
	headerLeft := lipgloss.NewStyle().Bold(true).Foreground(ColorPeach).Render("Atajos de teclado")
	spLen := max(1, innerW-lipgloss.Width(headerLeft)-lipgloss.Width(escBadge))
	header := headerLeft + strings.Repeat(" ", spLen) + escBadge

	colW := (innerW - 4) / 2
	sectionTitle := lipgloss.NewStyle().Bold(true).Foreground(ColorLavender)
	keyStyle := lipgloss.NewStyle().Foreground(ColorPeach).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(ColorText)

	// Columna Izquierda: Navegación & Modelos
	var leftCol strings.Builder
	leftCol.WriteString(sectionTitle.Render("Navegación") + "\n")
	leftCol.WriteString(fmt.Sprintf("  %-11s %s\n", keyStyle.Render("↑ / k"), descStyle.Render("Subir")))
	leftCol.WriteString(fmt.Sprintf("  %-11s %s\n", keyStyle.Render("↓ / j"), descStyle.Render("Bajar")))
	leftCol.WriteString(fmt.Sprintf("  %-11s %s\n", keyStyle.Render("p"), descStyle.Render("Fijar / Desanclar")))
	leftCol.WriteString(fmt.Sprintf("  %-11s %s\n", keyStyle.Render("P"), descStyle.Render("Limpiar fijados")))
	leftCol.WriteString(fmt.Sprintf("  %-11s %s\n", keyStyle.Render("Enter / l"), descStyle.Render("Ver logs")))
	leftCol.WriteString(fmt.Sprintf("  %-11s %s\n\n", keyStyle.Render("/"), descStyle.Render("Buscar / Filtrar")))

	leftCol.WriteString(sectionTitle.Render("Modelos de IA") + "\n")
	leftCol.WriteString(fmt.Sprintf("  %-11s %s\n", keyStyle.Render("c"), descStyle.Render("Elegir modelo")))
	leftCol.WriteString(fmt.Sprintf("  %-11s %s\n", keyStyle.Render("Ctrl+A"), descStyle.Render("Configurar API Key")))

	// Columna Derecha: Acciones & General
	var rightCol strings.Builder
	rightCol.WriteString(sectionTitle.Render("Acciones") + "\n")
	rightCol.WriteString(fmt.Sprintf("  %-11s %s\n", keyStyle.Render("r"), descStyle.Render("Reiniciar")))
	rightCol.WriteString(fmt.Sprintf("  %-11s %s\n", keyStyle.Render("s"), descStyle.Render("Detener")))
	rightCol.WriteString(fmt.Sprintf("  %-11s %s\n", keyStyle.Render("x"), descStyle.Render("Aislar de red")))
	rightCol.WriteString(fmt.Sprintf("  %-11s %s\n\n", keyStyle.Render("e"), descStyle.Render("Abrir terminal")))

	rightCol.WriteString(sectionTitle.Render("General") + "\n")
	rightCol.WriteString(fmt.Sprintf("  %-11s %s\n", keyStyle.Render("t"), descStyle.Render("Temas y estilos")))
	rightCol.WriteString(fmt.Sprintf("  %-11s %s\n", keyStyle.Render("?"), descStyle.Render("Ver esta ayuda")))
	rightCol.WriteString(fmt.Sprintf("  %-11s %s\n", keyStyle.Render("q / Esc"), descStyle.Render("Cerrar / Salir")))

	leftBlock := lipgloss.NewStyle().Width(colW).Render(leftCol.String())
	rightBlock := lipgloss.NewStyle().Width(colW).Render(rightCol.String())
	sep := "    "

	columnsRow := lipgloss.JoinHorizontal(lipgloss.Top, leftBlock, sep, rightBlock)
	centeredColumns := lipgloss.NewStyle().Width(innerW).Render(columnsRow)

	body := fmt.Sprintf("%s\n\n%s", header, centeredColumns)
	return StyleModal.Width(modalWidth).Render(body)
}
