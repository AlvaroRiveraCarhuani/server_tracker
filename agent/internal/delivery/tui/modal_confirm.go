package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) viewConfirmModal() string {
	actionStr := strings.ToUpper(string(m.pendingAction))
	title := StyleModalTitle.Render(fmt.Sprintf("Confirmar acción: %s", actionStr))

	question := fmt.Sprintf("¿Deseas ejecutar %s en el contenedor?", actionStr)

	modalWidth := 70
	if m.width > 20 && m.width-4 < modalWidth {
		modalWidth = m.width - 4
	}

	innerW := modalWidth - 6
	maxNameW := max(10, innerW-14)
	cName := truncate(m.pendingContainer.Name, maxNameW)

	containerLine := fmt.Sprintf("Contenedor : %s", lipgloss.NewStyle().Foreground(ColorText).Background(ColorSurface0).Bold(true).Render(cName))
	idLine := fmt.Sprintf("ID         : %s", lipgloss.NewStyle().Foreground(ColorSubtext0).Background(ColorSurface0).Render(m.pendingContainer.ID))

	btnBase := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderBackground(ColorSurface0).
		Width(24).
		Align(lipgloss.Center)

	var btnConfirmStyle, btnCancelStyle lipgloss.Style

	if m.confirmModalBtn == 0 {
		btnConfirmStyle = btnBase.
			BorderForeground(ColorGreen).
			Background(ColorGreen).
			Foreground(ColorBase).
			Bold(true)

		btnCancelStyle = btnBase.
			BorderForeground(ColorSurface2).
			Background(ColorSurface0).
			Foreground(ColorSubtext0)
	} else {
		btnConfirmStyle = btnBase.
			BorderForeground(ColorSurface2).
			Background(ColorSurface0).
			Foreground(ColorSubtext0)

		btnCancelStyle = btnBase.
			BorderForeground(ColorPeach).
			Background(ColorPeach).
			Foreground(ColorBase).
			Bold(true)
	}

	btnConfirm := btnConfirmStyle.Render("✔  CONFIRMAR (y)")
	btnCancel := btnCancelStyle.Render("✖  CANCELAR (n/Esc)")
	sep := lipgloss.NewStyle().Background(ColorSurface0).Render("      \n      \n      ")

	buttonsRow := lipgloss.JoinHorizontal(lipgloss.Top, btnConfirm, sep, btnCancel)
	centeredButtons := lipgloss.NewStyle().
		Width(innerW).
		Align(lipgloss.Center).
		Background(ColorSurface0).
		Render(buttonsRow)

	body := fmt.Sprintf("%s\n\n%s\n\n  %s\n  %s\n\n%s",
		title,
		question,
		containerLine,
		idLine,
		centeredButtons,
	)

	return StyleModal.Width(modalWidth).Render(body)
}
