package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// WrapByTokens realiza wrap de texto exclusivamente en fronteras de tokens (espacios en blanco).
// Prohibido cortar caracteres/runas al medio: preserva nombres de contenedores, IDs y flechas (->) intactos (F3).
func WrapByTokens(text string, maxWidth int) []string {
	if maxWidth <= 0 {
		return []string{text}
	}

	rawLines := strings.Split(text, "\n")
	var result []string

	for _, rawLine := range rawLines {
		if lipgloss.Width(rawLine) <= maxWidth {
			result = append(result, rawLine)
			continue
		}

		tokens := strings.Fields(rawLine)
		if len(tokens) == 0 {
			result = append(result, "")
			continue
		}

		var currentLine strings.Builder
		for _, token := range tokens {
			tokenWidth := lipgloss.Width(token)
			if currentLine.Len() == 0 {
				currentLine.WriteString(token)
				continue
			}

			// Probar si entra con un espacio previo
			candidateWidth := lipgloss.Width(currentLine.String()) + 1 + tokenWidth
			if candidateWidth <= maxWidth {
				currentLine.WriteString(" " + token)
			} else {
				// Emitir línea actual y comenzar la siguiente con el token completo
				result = append(result, currentLine.String())
				currentLine.Reset()
				currentLine.WriteString(token)
			}
		}

		if currentLine.Len() > 0 {
			result = append(result, currentLine.String())
		}
	}

	return result
}
