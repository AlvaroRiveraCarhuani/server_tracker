package tui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
	"github.com/alvaroriverac/server_tracker_agent/internal/core/service"
	"github.com/alvaroriverac/server_tracker_agent/internal/infrastructure/ai"
	tea "github.com/charmbracelet/bubbletea"
)

func truncate(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	return s
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// buildIncidentPrompt construye el prompt compacto para diagnosticar el incidente (Ítem 5).
func (m Model) buildIncidentPrompt(inc *service.Incident) string {
	if inc == nil {
		return ""
	}

	depGraph := service.NewDependencyGraph(m.metrics)
	candidateOrigin := inc.EffectiveOrigin()

	// Recopilar miembros únicos
	var members []domain.ContainerMetric
	seen := make(map[string]bool)
	for _, c := range inc.Members {
		clean := strings.TrimPrefix(c.Name, "/")
		if !seen[clean] {
			seen[clean] = true
			members = append(members, c)
		}
	}

	// Ordenar miembros por prioridad de cupo:
	// 1. Origen candidato primero
	// 2. Mayor número de dependientes
	// 3. Timestamp de evento
	sort.Slice(members, func(i, j int) bool {
		mI := strings.TrimPrefix(members[i].Name, "/")
		mJ := strings.TrimPrefix(members[j].Name, "/")
		if mI == candidateOrigin {
			return true
		}
		if mJ == candidateOrigin {
			return false
		}
		depsI := len(depGraph.GetDependents(members[i]))
		depsJ := len(depGraph.GetDependents(members[j]))
		if depsI != depsJ {
			return depsI > depsJ
		}
		return members[i].LastStateChange.Before(members[j].LastStateChange)
	})

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Incidente en grupo: %s (total %d contenedores involucrados)\n\n", inc.GroupName, len(members)))

	// Máximo 4 contenedores con logs + telemetría de 1 línea
	limit := 4
	if len(members) < limit {
		limit = len(members)
	}

	for i := 0; i < limit; i++ {
		c := members[i]
		clean := strings.TrimPrefix(c.Name, "/")
		ramMB := float64(c.RAMBytes) / (1024 * 1024)
		telemetryLine := fmt.Sprintf("Contenedor: %s | Estado: %s | CPU: %.1f%% | RAM: %.1fMB", clean, c.Status, c.CPUPercent, ramMB)
		b.WriteString(telemetryLine + "\n")

		// Obtener logs (últimas ~30 líneas o tail 30)
		logs, err := m.collector.GetContainerLogs(context.Background(), c.ID, 30)
		if err != nil || strings.TrimSpace(logs) == "" {
			b.WriteString("Logs: (sin logs disponibles)\n\n")
		} else {
			lines := strings.Split(strings.TrimSpace(logs), "\n")
			if len(lines) > 30 {
				lines = lines[len(lines)-30:]
			}
			b.WriteString("Logs:\n" + strings.Join(lines, "\n") + "\n\n")
		}
	}

	// Excedentes SOLO por nombre en contexto
	if len(members) > 4 {
		var others []string
		for i := 4; i < len(members); i++ {
			others = append(others, strings.TrimPrefix(members[i].Name, "/"))
		}
		b.WriteString(fmt.Sprintf("Otros contenedores involucrados en el incidente: %s\n\n", strings.Join(others, ", ")))
	}

	// Aristas relevantes del grafo etiquetadas (inferido)
	var edgeLines []string
	for _, m1 := range members {
		clean1 := strings.TrimPrefix(m1.Name, "/")
		for _, m2 := range members {
			clean2 := strings.TrimPrefix(m2.Name, "/")
			if clean1 != clean2 && depGraph.HasDependency(clean1, clean2) {
				edgeLines = append(edgeLines, fmt.Sprintf("arista %s -> %s (inferido)", clean1, clean2))
			}
		}
	}
	if len(edgeLines) > 0 {
		b.WriteString("Relaciones de dependencia detectadas:\n")
		for _, edge := range edgeLines {
			b.WriteString(" · " + edge + "\n")
		}
		b.WriteString("\n")
	}

	// Bloque de historial de Ola 4 del origen si existe
	if m.crashJournal != nil && candidateOrigin != "" {
		ramTrend := ""
		if hist := m.metricsHistory[candidateOrigin]; hist != nil {
			ramTrend = hist.CalculateRAMTrend()
		}
		if histBlock := m.crashJournal.FormatPromptBlock(candidateOrigin, ramTrend); histBlock != "" {
			b.WriteString("Historial del origen candidato:\n" + histBlock + "\n")
		}
	}

	return b.String()
}

func (m Model) triggerIncidentTriage(inc *service.Incident) tea.Cmd {
	return func() tea.Msg {
		if m.triageClient == nil || inc == nil {
			return nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()

		prompt := m.buildIncidentPrompt(inc)

		var raw string
		var usage domain.TokenUsage
		if tc, ok := m.triageClient.(*ai.TriageClient); ok {
			raw, usage = tc.DiagnoseIncident(ctx, prompt, domain.SlotDeep)
		} else {
			raw, usage = m.triageClient.DiagnoseContainerWithSlot(ctx, inc.GroupName, "incident", "critical", prompt, domain.SlotDeep)
		}

		parser := service.NewAIResponseParser()
		parsed := parser.Parse(raw, usage, "critical")

		validMembers := make(map[string]bool)
		for name := range inc.Members {
			validMembers[strings.TrimPrefix(name, "/")] = true
		}
		sanitized := parser.SanitizeIncidentMembers(parsed, validMembers)

		return incidentDiagnosisResultMsg{
			incidentID: inc.ID,
			result:     sanitized,
			usage:      usage,
		}
	}
}
