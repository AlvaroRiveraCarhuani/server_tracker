package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
	"github.com/alvaroriverac/server_tracker_agent/internal/core/service"
	"github.com/charmbracelet/lipgloss"
)

// EvidenceItem representa un elemento tipado de evidencia en el diagnóstico V4.
type EvidenceItem struct {
	Type  string // "evento", "métrica", "log", "historial"
	Label string // ej: "exit 137", "RAM", "CPU", "tendencia", "log"
	Value string // ej: "hace 2m", "508/512MB (99%)", "creciente ▁▃▅█", "Killed..."
}

// V4Action representa una acción ejecutable desde el overlay de diagnóstico V4.
type V4Action struct {
	Key        string
	ActionType domain.ActionType
	IsLogs     bool
	IsAI       bool
	Label      string
}

// BuildEvidence construye la lista tipada de evidencias a partir del contenedor y su diagnóstico.
func BuildEvidence(m Model, c domain.ContainerMetric, res domain.DiagnosisResult, logs string) []EvidenceItem {
	var items []EvidenceItem

	// 1. Evidencia de Evento: Código de salida y ciclo de vida
	exitCode := domain.ParseExitCode(c.Status)
	if exitCode >= 0 {
		val := "reciente"
		if !c.LastStateChange.IsZero() {
			val = formatDurationAgo(time.Since(c.LastStateChange))
		}
		items = append(items, EvidenceItem{
			Type:  "evento",
			Label: fmt.Sprintf("exit %d", exitCode),
			Value: val,
		})
	}

	if c.RestartCount > 0 {
		items = append(items, EvidenceItem{
			Type:  "evento",
			Label: "restarts",
			Value: fmt.Sprintf("%d en ciclo actual", c.RestartCount),
		})
	}

	// 2. Evidencia de Métrica: RAM con límites y porcentaje
	ramMB := float64(c.RAMBytes) / (1024 * 1024)
	if c.RAMLimitBytes > 0 {
		limitMB := float64(c.RAMLimitBytes) / (1024 * 1024)
		pct := (ramMB / limitMB) * 100.0
		items = append(items, EvidenceItem{
			Type:  "métrica",
			Label: "RAM",
			Value: fmt.Sprintf("%.0fMB / límite %.0fMB (%.0f%%)", ramMB, limitMB, pct),
		})
	} else if ramMB > 0 {
		items = append(items, EvidenceItem{
			Type:  "métrica",
			Label: "RAM",
			Value: fmt.Sprintf("%.1fMB (sin límite Docker)", ramMB),
		})
	}

	// 3. Evidencia de Métrica: CPU y Throttling
	cpuVal := fmt.Sprintf("%.1f%% · throttling %.0f%%", c.CPUPercent, c.CPUPercentThrottled)
	items = append(items, EvidenceItem{
		Type:  "métrica",
		Label: "CPU",
		Value: cpuVal,
	})

	// 4. Evidencia de Métrica: Tendencia y Sparkline
	if hist, ok := m.metricsHistory[c.ID]; ok && len(hist.CPU) > 0 {
		trend := hist.CalculateCPUTrend()
		spark := RenderSparkline(hist.CPU, 0, 100, 8)
		items = append(items, EvidenceItem{
			Type:  "métrica",
			Label: "tendencia",
			Value: fmt.Sprintf("%s  %s", trend.Label, spark),
		})
	}

	// 5. Evidencia de Log: Fragmento significativo
	logSnippet := strings.TrimSpace(logs)
	if logSnippet == "" && res.RawOutput != "" {
		logSnippet = strings.TrimSpace(res.RawOutput)
	}
	if logSnippet != "" {
		lines := strings.Split(logSnippet, "\n")
		lastLine := lines[len(lines)-1]
		if len(lastLine) > 55 {
			lastLine = lastLine[len(lastLine)-52:] + "..."
		}
		items = append(items, EvidenceItem{
			Type:  "log",
			Label: "log",
			Value: fmt.Sprintf("\"%s\"", lastLine),
		})
	}

	// 6. Evidencia de Red: Conflicto de puerto retenido por otro contenedor running
	graph := service.NewDependencyGraph(m.metrics)
	conflicts := graph.GetPortConflicts(c)
	isPortConflict := res.RootCause == "Conflicto de puerto en el host" ||
		strings.Contains(strings.ToLower(logs), "address already in use") ||
		strings.Contains(strings.ToLower(c.Status), "address already in use") ||
		strings.Contains(strings.ToLower(res.RawOutput), "address already in use") ||
		(domain.ParseExitCode(c.Status) == 1 && len(conflicts) > 0)

	if isPortConflict && len(conflicts) > 0 {
		ports := graph.GetPublishedPorts(c)
		for _, p := range ports {
			if owner, ok := conflicts[p.HostPort]; ok {
				items = append(items, EvidenceItem{
					Type:  "red",
					Label: fmt.Sprintf("puerto %d", p.HostPort),
					Value: fmt.Sprintf("retenido por: %s", owner),
				})
			}
		}
	}

	// 7. Evidencia de Historial (Ola 4)
	if m.crashJournal != nil {
		count1h := m.crashJournal.Count(c.ID, "", 1*time.Hour)
		if count1h == 0 && c.Name != "" {
			count1h = m.crashJournal.Count(c.Name, "", 1*time.Hour)
		}
		if count1h > 0 {
			homog := m.crashJournal.Homogeneity(c.ID, 1*time.Hour)
			if homog == "" && c.Name != "" {
				homog = m.crashJournal.Homogeneity(c.Name, 1*time.Hour)
			}
			lastEv := m.crashJournal.LastEvent(c.ID)
			if lastEv == nil && c.Name != "" {
				lastEv = m.crashJournal.LastEvent(c.Name)
			}
			lastAgo := "reciente"
			if lastEv != nil {
				lastAgo = formatDurationAgo(time.Since(lastEv.Timestamp))
			}
			items = append(items, EvidenceItem{
				Type:  "historial",
				Label: "historial",
				Value: fmt.Sprintf("%d crashes en 1h (%s) · último %s", count1h, homog, lastAgo),
			})

			prevDiag := m.crashJournal.PreviousDiagnosis(c.ID)
			if prevDiag == "" && c.Name != "" {
				prevDiag = m.crashJournal.PreviousDiagnosis(c.Name)
			}
			if prevDiag != "" {
				items = append(items, EvidenceItem{
					Type:  "historial",
					Label: "hipótesis previa",
					Value: prevDiag,
				})
			}

			hist := m.metricsHistory[c.ID]
			if hist == nil && c.Name != "" {
				hist = m.metricsHistory[c.Name]
			}
			if hist != nil && len(hist.RAM) > 0 {
				trend := hist.CalculateRAMTrend()
				if trend != "" {
					items = append(items, EvidenceItem{
						Type:  "historial",
						Label: "tendencia RAM",
						Value: trend,
					})
				}
			}
		}
	}

	// 8. Evidencia de Proceso e IA (Ola 5)
	if res.Confidence == "low" {
		items = append(items, EvidenceItem{
			Type:  "proceso",
			Label: "proceso",
			Value: "confianza del modelo: baja",
		})
	}
	if res.ReanalyzedDeep || res.ProcessNote != "" {
		note := res.ProcessNote
		if note == "" {
			note = "re-analizado en DEEP por severidad crítica"
		}
		items = append(items, EvidenceItem{
			Type:  "proceso",
			Label: "proceso",
			Value: note,
		})
	}

	return items
}

// GetV4Actions obtiene la lista de acciones disponibles para el contenedor en V4.
func (m Model) GetV4Actions(c domain.ContainerMetric, res domain.DiagnosisResult) []V4Action {
	var actions []V4Action

	// Acción sugerida según regla o IA
	switch res.SuggestedAction {
	case "restart":
		actions = append(actions, V4Action{
			Key:        "r",
			ActionType: domain.ActionRestart,
			Label:      "aplicar restart",
		})
	case "stop":
		actions = append(actions, V4Action{
			Key:        "s",
			ActionType: domain.ActionStop,
			Label:      "aplicar stop",
		})
	case "isolate":
		actions = append(actions, V4Action{
			Key:        "x",
			ActionType: domain.ActionIsolateNetwork,
			Label:      "aislar de red",
		})
	}

	// Logs siempre disponible
	actions = append(actions, V4Action{
		Key:    "l",
		IsLogs: true,
		Label:  "ver logs",
	})

	// Solicitar IA si está en modo MANUAL o si el nivel es [RULE] o [SIG] o [AI~]
	if m.aiConfig.SelectionMode == domain.SelectionManual || res.Level == domain.LevelRule || res.Level == domain.LevelSignal || res.Level == domain.LevelAIPartial {
		actions = append(actions, V4Action{
			Key:   "i",
			IsAI:  true,
			Label: "solicitar diagnóstico IA",
		})
	}

	return actions
}

// viewDiagnosisModal renderiza el overlay centrado V4 de diagnóstico y observabilidad.
func (m Model) viewDiagnosisModal() string {
	if m.activeIncident != nil {
		return m.viewIncidentDiagnosisModal()
	}

	modalWidth := 74
	if m.width > 20 && m.width-4 < modalWidth {
		modalWidth = m.width - 4
	}
	innerW := modalWidth - 6

	bgStyle := lipgloss.NewStyle().Background(ColorSurface0)
	headerLeft := lipgloss.NewStyle().Bold(true).Foreground(ColorPeach).Background(ColorSurface0).Render(fmt.Sprintf("diagnóstico · %s", m.selectedName))

	var lines []string

	// 1. Diagnóstico Principal (Tag + Causa Raíz)
	res, hasRes := m.diagnosisResults[m.selectedID]
	var tagStyled string
	rootCause := "Sin diagnóstico concluyente"

	if hasRes {
		rootCause = res.RootCause
		switch res.Level {
		case domain.LevelAI:
			tagStyled = StyleTagAI.Render("[AI]")
		case domain.LevelAIPartial:
			tagStyled = StyleTagAIPartial.Render("[AI~]")
		case domain.LevelRule:
			tagStyled = StyleTagRule.Render("[RULE]")
		default:
			tagStyled = StyleTagSignal.Render("[SIG]")
		}
	} else {
		tagStyled = StyleTagSignal.Render("[SIG]")
		if exitCode := domain.ParseExitCode(m.selectedState); exitCode >= 0 {
			rootCause = fmt.Sprintf("Exit code %d · sin diagnóstico", exitCode)
		} else {
			rootCause = fmt.Sprintf("%s · sin diagnóstico", m.selectedState)
		}
	}

	diagLine := fmt.Sprintf("%s %s", tagStyled, lipgloss.NewStyle().Foreground(ColorText).Bold(true).Render(rootCause))
	lines = append(lines, diagLine)
	if res.RecurrenceNote != "" {
		noteStyled := lipgloss.NewStyle().Foreground(ColorPeach).Render(fmt.Sprintf("  %s", res.RecurrenceNote))
		lines = append(lines, noteStyled)
	}
	lines = append(lines, "")

	// 2. Sección de Evidencia Tipada (con soporte de mini-scroll por Tab F3)
	evidences := BuildEvidence(m, m.pendingContainer, res, "")
	compact := m.height < 30
	needsScroll := compact && len(evidences) > 4

	evidenceTitle := "evidencia:"
	titleColor := ColorLavender
	if needsScroll {
		if m.v4FocusSection == 1 {
			evidenceTitle = "evidencia [activo • ↑/↓ para scrollear]:"
			titleColor = ColorPeach
		} else {
			evidenceTitle = "evidencia [pulsa Tab para scrollear]:"
		}
	}
	lines = append(lines, lipgloss.NewStyle().Foreground(titleColor).Bold(true).Render(evidenceTitle))

	if len(evidences) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(ColorSubtext0).Render("  · sin telemetría anómala registrada"))
	} else if !needsScroll {
		for _, ev := range evidences {
			dot := lipgloss.NewStyle().Foreground(ColorSubtext1).Render("  ·")
			if ev.Type == "red" {
				label := lipgloss.NewStyle().Foreground(ColorSubtext0).Render(ev.Label)
				val := lipgloss.NewStyle().Foreground(ColorText).Render(ev.Value)
				lines = append(lines, fmt.Sprintf("%s %s · %s", dot, label, val))
			} else {
				label := lipgloss.NewStyle().Foreground(ColorSubtext0).Render(ev.Label + ":")
				val := lipgloss.NewStyle().Foreground(ColorText).Render(ev.Value)
				lines = append(lines, fmt.Sprintf("%s %s %s", dot, label, val))
			}
		}
	} else {
		windowSize := 4
		maxScroll := len(evidences) - windowSize
		offset := max(0, min(m.v4EvidenceScroll, maxScroll))

		if offset > 0 {
			lines = append(lines, lipgloss.NewStyle().Foreground(ColorSubtext0).Render("  · ↑ más arriba"))
		}

		end := offset + windowSize
		if end > len(evidences) {
			end = len(evidences)
		}

		for i := offset; i < end; i++ {
			ev := evidences[i]
			dot := lipgloss.NewStyle().Foreground(ColorSubtext1).Render("  ·")
			if ev.Type == "red" {
				label := lipgloss.NewStyle().Foreground(ColorSubtext0).Render(ev.Label)
				val := lipgloss.NewStyle().Foreground(ColorText).Render(ev.Value)
				lines = append(lines, fmt.Sprintf("%s %s · %s", dot, label, val))
			} else {
				label := lipgloss.NewStyle().Foreground(ColorSubtext0).Render(ev.Label + ":")
				val := lipgloss.NewStyle().Foreground(ColorText).Render(ev.Value)
				lines = append(lines, fmt.Sprintf("%s %s %s", dot, label, val))
			}
		}

		remaining := len(evidences) - end
		if remaining > 0 {
			if m.v4FocusSection == 1 {
				lines = append(lines, lipgloss.NewStyle().Foreground(ColorPeach).Render(
					fmt.Sprintf("  · ↓ +%d más (usa ↓ para bajar)", remaining),
				))
			} else {
				lines = append(lines, lipgloss.NewStyle().Foreground(ColorSubtext0).Render(
					fmt.Sprintf("  · +%d más · pulsa [Tab] para scrollear", remaining),
				))
			}
		}
	}
	lines = append(lines, "")

	// 3. Acciones Sugeridas Navegables
	actionTitle := "acción sugerida:"
	if needsScroll && m.v4FocusSection == 0 {
		actionTitle = "acción sugerida [activo]:"
	}
	lines = append(lines, lipgloss.NewStyle().Foreground(ColorLavender).Bold(true).Render(actionTitle))
	actions := m.GetV4Actions(m.pendingContainer, res)

	for i, act := range actions {
		isCursor := i == m.v4ActionCursor && m.v4FocusSection == 0
		var row string
		if isCursor {
			ptr := lipgloss.NewStyle().Foreground(ColorPeach).Bold(true).Render(">")
			key := lipgloss.NewStyle().Foreground(ColorPeach).Bold(true).Render(fmt.Sprintf("[%s]", act.Key))
			lbl := lipgloss.NewStyle().Foreground(ColorText).Bold(true).Render(act.Label)
			row = fmt.Sprintf("  %s %s %s", ptr, key, lbl)
		} else {
			key := lipgloss.NewStyle().Foreground(ColorSubtext1).Render(fmt.Sprintf("[%s]", act.Key))
			lbl := lipgloss.NewStyle().Foreground(ColorSubtext0).Render(act.Label)
			row = fmt.Sprintf("    %s %s", key, lbl)
		}
		lines = append(lines, row)
	}

	lines = append(lines, "")
	var footerHint string
	if needsScroll {
		if m.v4FocusSection == 1 {
			footerHint = lipgloss.NewStyle().Foreground(ColorPeach).Render("↑/↓: scrollear evidencia  ·  tab: ir a acciones  ·  esc: volver")
		} else {
			footerHint = lipgloss.NewStyle().Foreground(ColorSubtext0).Render("enter: ejecutar  ·  tab: scrollear evidencia  ·  ↑/↓: seleccionar  ·  esc: volver")
		}
	} else {
		footerHint = lipgloss.NewStyle().Foreground(ColorSubtext0).Render("enter: ejecutar  ·  esc: volver  ·  ↑/↓: seleccionar")
	}
	lines = append(lines, footerHint)

	// Scroll y Viewport (F3)
	availH := max(6, m.height-6)
	displayedLines := lines
	scrollBadge := ""
	if len(lines) > availH {
		maxOffset := len(lines) - availH
		offset := max(0, min(m.overlayScrollOffset, maxOffset))
		displayedLines = lines[offset : offset+availH]
		scrollBadge = fmt.Sprintf("(%d/%d) ", offset+1, maxOffset+1)
	}

	escBadge := lipgloss.NewStyle().Foreground(ColorSubtext0).Background(ColorSurface0).Render(scrollBadge + "esc")
	spLen := max(1, innerW-lipgloss.Width(headerLeft)-lipgloss.Width(escBadge))
	header := headerLeft + bgStyle.Render(strings.Repeat(" ", spLen)) + escBadge

	body := fmt.Sprintf("%s\n\n%s", header, strings.Join(displayedLines, "\n"))
	return StyleModal.Width(modalWidth).Render(body)
}

func formatDurationAgo(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("hace %ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("hace %dm", int(d.Minutes()))
	}
	return fmt.Sprintf("hace %dh", int(d.Hours()))
}

func (m Model) getIncidentContainersList() []string {
	if m.activeIncident == nil {
		return nil
	}
	origin := m.activeIncident.EffectiveOrigin()
	var list []string
	seen := make(map[string]bool)
	if origin != "" {
		list = append(list, origin)
		seen[origin] = true
	}
	for _, c := range m.activeIncident.Cascade {
		if !seen[c] {
			list = append(list, c)
			seen[c] = true
		}
	}
	for _, member := range m.activeIncident.Members {
		clean := strings.TrimPrefix(member.Name, "/")
		if !seen[clean] {
			list = append(list, clean)
			seen[clean] = true
		}
	}
	return list
}

// viewIncidentDiagnosisModal renderiza la superficie V4 en modo incidente (Decisión I5).
func (m Model) viewIncidentDiagnosisModal() string {
	inc := m.activeIncident
	if inc == nil {
		return ""
	}

	modalWidth := 74
	if m.width > 20 && m.width-4 < modalWidth {
		modalWidth = m.width - 4
	}
	innerW := modalWidth - 6

	bgStyle := lipgloss.NewStyle().Background(ColorSurface0)
	headerLeft := lipgloss.NewStyle().Bold(true).Foreground(ColorPeach).Background(ColorSurface0).Render(fmt.Sprintf("diagnóstico · incidente %s", inc.GroupName))

	var lines []string

	// 1. Diagnóstico Principal (Tag + Causa Raíz)
	tagStyled := StyleTagIncident.Render("[INC]")
	sourceTag := StyleTagRule.Render("[RULE]")
	if inc.Diagnosis.Level == domain.LevelAI {
		sourceTag = StyleTagAI.Render("[AI]")
	} else if inc.Diagnosis.Level == domain.LevelAIPartial {
		sourceTag = StyleTagAIPartial.Render("[AI~]")
	}

	rootCause := inc.Diagnosis.RootCause
	if rootCause == "" {
		rootCause = fmt.Sprintf("Cascada en %s", inc.GroupName)
	}

	titleRowLeft := fmt.Sprintf("%s %s", tagStyled, lipgloss.NewStyle().Foreground(ColorText).Bold(true).Render(rootCause))
	spTitle := max(1, innerW-lipgloss.Width(titleRowLeft)-lipgloss.Width(sourceTag))
	titleRow := titleRowLeft + strings.Repeat(" ", spTitle) + sourceTag
	lines = append(lines, titleRow)
	lines = append(lines, "")

	// 2. Bloque Origen
	originName := inc.EffectiveOrigin()
	var originStatusLabel string
	if inc.ManualOrigin != "" {
		originStatusLabel = fmt.Sprintf("%s (manual)", originName)
	} else {
		switch inc.OriginConfidence {
		case domain.ConfidenceConfirmed:
			originStatusLabel = fmt.Sprintf("%s (confirmado)", originName)
		case domain.ConfidenceProbable:
			originStatusLabel = fmt.Sprintf("%s (probable)", originName)
		default:
			originStatusLabel = "sin determinar · señales en conflicto"
		}
	}

	// Historial / Recurrencia del origen (Ola 4)
	if m.crashJournal != nil && originName != "" {
		c1h := m.crashJournal.Count(originName, "", 1*time.Hour)
		if c1h >= 2 {
			originStatusLabel += fmt.Sprintf(" (%d crashes en 1h)", c1h)
		}
	}

	lines = append(lines, fmt.Sprintf("%s %s",
		lipgloss.NewStyle().Foreground(ColorLavender).Bold(true).Render("origen:"),
		lipgloss.NewStyle().Foreground(ColorText).Render(originStatusLabel),
	))

	if inc.OriginConfidence == domain.ConfidenceUndetermined && inc.ManualOrigin == "" {
		lines = append(lines, lipgloss.NewStyle().Foreground(ColorSubtext0).Render("  · [o] sobre un contenedor para marcar origen manual"))
	} else {
		for _, ev := range inc.OriginEvidence {
			lines = append(lines, lipgloss.NewStyle().Foreground(ColorSubtext0).Render(fmt.Sprintf("  · %s", ev)))
		}
	}
	lines = append(lines, "")

	// 3. Bloque Timeline (F3 modo compacto: últimos 3 + +N más)
	compact := m.height < 30
	lines = append(lines, lipgloss.NewStyle().Foreground(ColorLavender).Bold(true).Render("timeline:"))
	if len(inc.Events) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(ColorSubtext0).Render("  · sin eventos registrados"))
	} else {
		eventsToShow := inc.Events
		if compact && len(inc.Events) > 3 {
			eventsToShow = inc.Events[len(inc.Events)-3:]
		}
		for _, ev := range eventsToShow {
			timeStr := ev.Timestamp.Format("15:04:05")
			reasonStr := ev.Reason
			if reasonStr == "" {
				reasonStr = ev.Status
			}
			lines = append(lines, fmt.Sprintf("  · %s %s %s",
				lipgloss.NewStyle().Foreground(ColorSubtext1).Render(timeStr),
				lipgloss.NewStyle().Foreground(ColorText).Bold(true).Render(ev.ContainerName),
				lipgloss.NewStyle().Foreground(ColorSubtext0).Render(reasonStr),
			))
		}
		if compact && len(inc.Events) > 3 {
			lines = append(lines, lipgloss.NewStyle().Foreground(ColorSubtext0).Render(fmt.Sprintf("  · +%d más", len(inc.Events)-3)))
		}
	}

	// 4. Bloque Cascada con Wrap por Tokens (F3)
	if len(inc.Cascade) > 0 {
		cascadeStr := strings.Join(inc.Cascade, " -> ") + " (inferido)"
		wrapped := WrapByTokens(cascadeStr, max(20, innerW-10))
		for i, cw := range wrapped {
			if i == 0 {
				lines = append(lines, fmt.Sprintf("%s %s",
					lipgloss.NewStyle().Foreground(ColorLavender).Bold(true).Render("cascada:"),
					lipgloss.NewStyle().Foreground(ColorText).Render(cw),
				))
			} else {
				lines = append(lines, fmt.Sprintf("         %s", lipgloss.NewStyle().Foreground(ColorText).Render(cw)))
			}
		}
	}
	lines = append(lines, "")

	// 5. Bloque Contenedores Navegables e Indicador de Vecinos Sanos (F4)
	containersList := m.getIncidentContainersList()
	lines = append(lines, lipgloss.NewStyle().Foreground(ColorLavender).Bold(true).Render("contenedores:"))
	hasActionRow := (inc.ManualOrigin != "" || inc.OriginConfidence != domain.ConfidenceUndetermined) && originName != ""

	for i, cName := range containersList {
		isCursor := (i == m.v4IncidentCursor)
		enterBadge := lipgloss.NewStyle().Foreground(ColorSubtext1).Render("[Enter] diagnóstico")
		oBadge := lipgloss.NewStyle().Foreground(ColorSubtext1).Render("[o] origen")
		if isCursor {
			enterBadge = lipgloss.NewStyle().Foreground(ColorPeach).Bold(true).Render("[Enter] diagnóstico")
			oBadge = lipgloss.NewStyle().Foreground(ColorPeach).Bold(true).Render("[o] origen")
		}

		var row string
		if isCursor {
			ptr := lipgloss.NewStyle().Foreground(ColorPeach).Bold(true).Render(">")
			nameStyled := lipgloss.NewStyle().Foreground(ColorText).Bold(true).Render(fmt.Sprintf("%-14s", cName))
			row = fmt.Sprintf("  %s %s  %s  %s", ptr, nameStyled, enterBadge, oBadge)
		} else {
			nameStyled := lipgloss.NewStyle().Foreground(ColorSubtext0).Render(fmt.Sprintf("%-14s", cName))
			row = fmt.Sprintf("    %s  %s  %s", nameStyled, enterBadge, oBadge)
		}
		lines = append(lines, row)
	}

	healthyCount := service.CountHealthyPeers(inc.GroupName, m.metrics)
	if healthyCount > 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(ColorSubtext0).Render(
			fmt.Sprintf("  otros en %s: %d · sin anomalías", inc.GroupName, healthyCount),
		))
	}
	lines = append(lines, "")

	// 6. Bloque Acción Sugerida
	if hasActionRow {
		lines = append(lines, lipgloss.NewStyle().Foreground(ColorLavender).Bold(true).Render("acción sugerida:"))
		isActionCursor := (m.v4IncidentCursor == len(containersList))
		actKey := "r"
		actVerb := "restart"
		if inc.Diagnosis.SuggestedAction == "stop" {
			actKey = "s"
			actVerb = "stop"
		} else if inc.Diagnosis.SuggestedAction == "isolate" {
			actKey = "x"
			actVerb = "isolate"
		}

		var row string
		if isActionCursor {
			ptr := lipgloss.NewStyle().Foreground(ColorPeach).Bold(true).Render(">")
			keyBadge := lipgloss.NewStyle().Foreground(ColorPeach).Bold(true).Render(fmt.Sprintf("[%s]", actKey))
			actionText := lipgloss.NewStyle().Foreground(ColorText).Bold(true).Render(fmt.Sprintf("aplicar %s a %s (origen)", actVerb, originName))
			row = fmt.Sprintf("  %s %s %s", ptr, keyBadge, actionText)
		} else {
			keyBadge := lipgloss.NewStyle().Foreground(ColorSubtext1).Render(fmt.Sprintf("[%s]", actKey))
			actionText := lipgloss.NewStyle().Foreground(ColorSubtext0).Render(fmt.Sprintf("aplicar %s a %s (origen)", actVerb, originName))
			row = fmt.Sprintf("    %s %s", keyBadge, actionText)
		}
		lines = append(lines, row)
		lines = append(lines, "")
	}

	footerHint := lipgloss.NewStyle().Foreground(ColorSubtext0).Render("enter: ejecutar  ·  esc: volver  ·  ↑/↓: seleccionar")
	lines = append(lines, footerHint)

	// Scroll y Viewport (F3)
	availH := max(6, m.height-6)
	displayedLines := lines
	scrollBadge := ""
	if len(lines) > availH {
		maxOffset := len(lines) - availH
		offset := max(0, min(m.overlayScrollOffset, maxOffset))
		displayedLines = lines[offset : offset+availH]
		scrollBadge = fmt.Sprintf("(%d/%d) ", offset+1, maxOffset+1)
	}

	escBadge := lipgloss.NewStyle().Foreground(ColorSubtext0).Background(ColorSurface0).Render(scrollBadge + "esc")
	spLen := max(1, innerW-lipgloss.Width(headerLeft)-lipgloss.Width(escBadge))
	header := headerLeft + bgStyle.Render(strings.Repeat(" ", spLen)) + escBadge

	body := fmt.Sprintf("%s\n\n%s", header, strings.Join(displayedLines, "\n"))
	return StyleModal.Width(modalWidth).Render(body)
}
