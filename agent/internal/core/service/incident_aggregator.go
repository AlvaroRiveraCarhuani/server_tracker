package service

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
	"github.com/alvaroriverac/server_tracker_agent/internal/i18n"
)

// IncidentEvent registra un evento anómalo dentro del timeline del incidente.
type IncidentEvent struct {
	Timestamp     time.Time `json:"timestamp"`
	ContainerID   string    `json:"container_id"`
	ContainerName string    `json:"container_name"`
	Status        string    `json:"status"`
	Reason        string    `json:"reason"`
}

// Incident representa un conjunto correlacionado de anomalías por topología dura.
type Incident struct {
	ID                string                            `json:"id"`
	GroupName         string                            `json:"group_name"`
	Members           map[string]domain.ContainerMetric `json:"members"` // ID o Name -> Metric
	Events            []IncidentEvent                   `json:"events"`
	CandidateOrigin   string                            `json:"candidate_origin"`
	OriginConfidence  domain.OriginConfidence           `json:"origin_confidence"` // "confirmado", "probable", "s/d"
	OriginEvidence    []string                          `json:"origin_evidence"`
	Cascade           []string                          `json:"cascade"`
	ManualOrigin      string                            `json:"manual_origin,omitempty"`
	Diagnosis         domain.DiagnosisResult            `json:"diagnosis"`
	OpenedAt          time.Time                         `json:"opened_at"`
	LastAdjunctionAt  time.Time                         `json:"last_adjunction_at"`
	QuietUntil        time.Time                         `json:"quiet_until"`
	Closed            bool                              `json:"closed"`
	DiagnoseCount     int                               `json:"diagnose_count"`
	HasFinalDiagnosis bool                              `json:"has_final_diagnosis"`
}

// EffectiveOrigin retorna el contenedor origen efectivo considerando el override manual.
func (inc *Incident) EffectiveOrigin() string {
	if inc.ManualOrigin != "" {
		return inc.ManualOrigin
	}
	return inc.CandidateOrigin
}

// IncidentAggregator gestiona la correlación, adjunción y cierre de incidentes en memoria.
type IncidentAggregator struct {
	mu             sync.RWMutex
	incidents      map[string]*Incident        // incidentID -> Incident
	memberIncident map[string]string           // containerID y Name -> incidentID
	recentEvents   map[string]IncidentEvent    // containerName -> último evento anómalo registrado
	windowDuration time.Duration               // default 30s
	ruleEngine     *RuleEngine
}

// NewIncidentAggregator inicializa el agregador de incidentes con la ventana configurada.
func NewIncidentAggregator(windowSeconds int) *IncidentAggregator {
	if windowSeconds <= 0 {
		windowSeconds = 30
	}
	return &IncidentAggregator{
		incidents:      make(map[string]*Incident),
		memberIncident: make(map[string]string),
		recentEvents:   make(map[string]IncidentEvent),
		windowDuration: time.Duration(windowSeconds) * time.Second,
		ruleEngine:     NewRuleEngine(),
	}
}

// SetWindowSeconds actualiza la ventana de adjunción en caliente.
func (a *IncidentAggregator) SetWindowSeconds(sec int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if sec > 0 {
		a.windowDuration = time.Duration(sec) * time.Second
	}
}

// GetActiveIncidentFor retorna el incidente activo al que pertenece un contenedor, o nil.
func (a *IncidentAggregator) GetActiveIncidentFor(containerIDOrName string) *Incident {
	a.mu.RLock()
	defer a.mu.RUnlock()

	clean := strings.TrimPrefix(containerIDOrName, "/")
	incID, ok := a.memberIncident[clean]
	if !ok {
		incID, ok = a.memberIncident[containerIDOrName]
	}
	if !ok {
		return nil
	}
	inc := a.incidents[incID]
	if inc != nil && !inc.Closed {
		return inc
	}
	return nil
}

// GetAllActiveIncidents retorna la lista de incidentes activos.
func (a *IncidentAggregator) GetAllActiveIncidents() []*Incident {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var result []*Incident
	for _, inc := range a.incidents {
		if !inc.Closed {
			result = append(result, inc)
		}
	}
	return result
}

// IngestAnomalies procesa la telemetría actual, identifica componentes conectados duros
// y gestiona aperturas y adjunciones según las reglas I1 e I2.
func (a *IncidentAggregator) IngestAnomalies(
	metrics []domain.ContainerMetric,
	graph *DependencyGraph,
	journal *CrashJournal,
	now time.Time,
) []*Incident {
	a.mu.Lock()
	defer a.mu.Unlock()

	// 1. Filtrar contenedores anómalos
	var anomalous []domain.ContainerMetric
	anomMap := make(map[string]domain.ContainerMetric)
	for _, c := range metrics {
		if domain.IsAnomalous(c) {
			anomalous = append(anomalous, c)
			clean := strings.TrimPrefix(c.Name, "/")
			anomMap[clean] = c
			anomMap[c.ID] = c

			// Registrar evento reciente si no existe
			if _, exists := a.recentEvents[clean]; !exists {
				reason := extractAnomalyReason(c)
				t := c.LastStateChange
				if t.IsZero() {
					t = now
				}
				a.recentEvents[clean] = IncidentEvent{
					Timestamp:     t,
					ContainerID:   c.ID,
					ContainerName: clean,
					Status:        c.Status,
					Reason:        reason,
				}
			}
		}
	}

	var newlyOpenedOrUpdated []*Incident

	// 2. Evaluar adjunciones a incidentes existentes (I2)
	for _, c := range anomalous {
		cleanName := strings.TrimPrefix(c.Name, "/")
		if existingID, ok := a.memberIncident[cleanName]; ok {
			if inc, found := a.incidents[existingID]; found && !inc.Closed {
				// Ya es miembro del incidente activo
				inc.Members[c.ID] = c
				continue
			}
		}

		// Buscar si puede adjuntarse a algún incidente activo
		for _, inc := range a.incidents {
			if inc.Closed {
				continue
			}

			// Verificar enlace por topología dura con algún miembro del incidente (I1)
			hasHardTopology := false
			for _, m := range inc.Members {
				if shareHardTopology(c, m) {
					hasHardTopology = true
					break
				}
			}

			if !hasHardTopology {
				continue
			}

			// Regla I2: adjunción dentro de ventana O por arista depende-de con incidente abierto < 5m
			canAdjoin := false
			withinWindow := now.Sub(inc.LastAdjunctionAt) <= a.windowDuration
			hasEdge := false
			if graph != nil {
				for _, m := range inc.Members {
					mClean := strings.TrimPrefix(m.Name, "/")
					if graph.HasDependency(cleanName, mClean) || graph.HasDependency(mClean, cleanName) {
						hasEdge = true
						break
					}
				}
			}
			openLessThan5m := now.Sub(inc.OpenedAt) < 5*time.Minute

			if withinWindow || (hasEdge && openLessThan5m) {
				canAdjoin = true
			}

			if canAdjoin {
				// Adjuntar al incidente
				inc.Members[c.ID] = c
				a.memberIncident[cleanName] = inc.ID
				a.memberIncident[c.ID] = inc.ID

				ev, ok := a.recentEvents[cleanName]
				if !ok {
					t := c.LastStateChange
					if t.IsZero() {
						t = now
					}
					ev = IncidentEvent{
						Timestamp:     t,
						ContainerID:   c.ID,
						ContainerName: cleanName,
						Status:        c.Status,
						Reason:        extractAnomalyReason(c),
					}
				}
				inc.Events = append(inc.Events, ev)
				inc.LastAdjunctionAt = now
				inc.QuietUntil = now.Add(2 * a.windowDuration)

				a.recomputeIncident(inc, graph, journal)
				newlyOpenedOrUpdated = append(newlyOpenedOrUpdated, inc)
				break
			}
		}
	}

	// 3. Evaluar formación de nuevos incidentes (mínimo 2 anomalías enlazadas por topología dura)
	// Identificar componentes conectados entre las anomalías aún no asignadas
	var unassigned []domain.ContainerMetric
	for _, c := range anomalous {
		cleanName := strings.TrimPrefix(c.Name, "/")
		if incID, ok := a.memberIncident[cleanName]; !ok || a.incidents[incID].Closed {
			unassigned = append(unassigned, c)
		}
	}

	components := buildHardTopologyComponents(unassigned)
	for _, comp := range components {
		if len(comp) < 2 {
			// Coincidencia o anomalía aislada sin lazo topológico -> no forma incidente
			continue
		}

		// Crear nuevo incidente
		groupName := resolveGroupName(comp)
		incID := fmt.Sprintf("inc-%s-%d", groupName, now.UnixNano())
		inc := &Incident{
			ID:               incID,
			GroupName:        groupName,
			Members:          make(map[string]domain.ContainerMetric),
			OpenedAt:         now,
			LastAdjunctionAt: now,
			QuietUntil:       now.Add(2 * a.windowDuration),
			Closed:           false,
		}

		for _, c := range comp {
			cleanName := strings.TrimPrefix(c.Name, "/")
			inc.Members[c.ID] = c
			a.memberIncident[cleanName] = incID
			a.memberIncident[c.ID] = incID

			ev, ok := a.recentEvents[cleanName]
			if !ok {
				t := c.LastStateChange
				if t.IsZero() {
					t = now
				}
				ev = IncidentEvent{
					Timestamp:     t,
					ContainerID:   c.ID,
					ContainerName: cleanName,
					Status:        c.Status,
					Reason:        extractAnomalyReason(c),
				}
			}
			inc.Events = append(inc.Events, ev)
		}

		a.incidents[incID] = inc
		a.recomputeIncident(inc, graph, journal)
		newlyOpenedOrUpdated = append(newlyOpenedOrUpdated, inc)
	}

	return newlyOpenedOrUpdated
}

// CheckQuietPeriods verifica y procesa cierres de incidentes tras 2x ventana sin adjunciones (I2).
func (a *IncidentAggregator) CheckQuietPeriods(now time.Time) (closing []*Incident) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for _, inc := range a.incidents {
		if inc.Closed {
			continue
		}
		if now.After(inc.QuietUntil) || now.Equal(inc.QuietUntil) {
			// Período quieto finalizado: cerrar incidente
			inc.Closed = true
			for id, m := range inc.Members {
				delete(a.memberIncident, id)
				clean := strings.TrimPrefix(m.Name, "/")
				delete(a.memberIncident, clean)
				delete(a.recentEvents, id)
				delete(a.recentEvents, clean)
			}
			closing = append(closing, inc)
		}
	}
	return closing
}

// SetManualOrigin permite al operador forzar el origen de un incidente mediante la tecla 'o' (I9).
func (a *IncidentAggregator) SetManualOrigin(incidentID, containerName string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	inc, ok := a.incidents[incidentID]
	if !ok || inc.Closed {
		return false
	}
	clean := strings.TrimPrefix(containerName, "/")
	found := false
	var targetName string
	for id, m := range inc.Members {
		if id == containerName || strings.TrimPrefix(m.Name, "/") == clean {
			found = true
			targetName = strings.TrimPrefix(m.Name, "/")
			break
		}
	}
	if !found {
		return false
	}
	inc.ManualOrigin = targetName
	return true
}

// recomputeIncident recalcula el timeline, origen candidato, confianza y cascada.
func (a *IncidentAggregator) recomputeIncident(inc *Incident, graph *DependencyGraph, journal *CrashJournal) {
	// 1. Ordenar eventos por timestamp ascendente
	sort.Slice(inc.Events, func(i, j int) bool {
		return inc.Events[i].Timestamp.Before(inc.Events[j].Timestamp)
	})

	if len(inc.Events) == 0 {
		return
	}

	oldest := inc.Events[0]
	inc.CandidateOrigin = oldest.ContainerName

	// 2. Construir Cascada preliminar por timeline
	var cascade []string
	seen := make(map[string]bool)
	cascade = append(cascade, inc.CandidateOrigin)
	seen[inc.CandidateOrigin] = true

	for _, ev := range inc.Events {
		if !seen[ev.ContainerName] {
			cascade = append(cascade, ev.ContainerName)
			seen[ev.ContainerName] = true
		}
	}
	inc.Cascade = cascade

	// 3. Evaluar confianza del origen según Decisión I7
	// confirmado = dos señales independientes coinciden (timeline más antiguo + arista grafo agreeing)
	// probable   = una sola señal (timeline solo)
	// s/d        = señales en conflicto o ninguna (dos simultáneos sin arista)
	var evidences []string
	evidences = append(evidences, fmt.Sprintf("evento más antiguo %s", oldest.Timestamp.Format("15:04:05")))

	hasAgreeingEdge := false
	var edgeDesc string
	if graph != nil {
		for _, ev := range inc.Events[1:] {
			if graph.HasDependency(ev.ContainerName, oldest.ContainerName) {
				hasAgreeingEdge = true
				edgeDesc = fmt.Sprintf("arista %s -> %s (inferido)", ev.ContainerName, oldest.ContainerName)
				break
			}
		}
	}

	// Verificar si hay eventos casi simultáneos (<= 1s de diferencia) sin arista que los desempate
	isSimultaneousConflict := false
	if len(inc.Events) >= 2 {
		diff := inc.Events[1].Timestamp.Sub(inc.Events[0].Timestamp)
		if diff < 0 {
			diff = -diff
		}
		if diff <= 1*time.Second && !hasAgreeingEdge {
			isSimultaneousConflict = true
		}
	}

	if isSimultaneousConflict {
		inc.OriginConfidence = domain.ConfidenceUndetermined
		inc.OriginEvidence = []string{"señales en conflicto: eventos simultáneos sin dependencias"}
	} else if hasAgreeingEdge {
		inc.OriginConfidence = domain.ConfidenceConfirmed
		evidences = append(evidences, edgeDesc)
		inc.OriginEvidence = evidences
	} else {
		inc.OriginConfidence = domain.ConfidenceProbable
		inc.OriginEvidence = evidences
	}

	// 4. Si el diagnóstico actual es de reglas locales o vacío, generar diagnóstico determinístico I4
	if inc.Diagnosis.Level == "" || inc.Diagnosis.Level == domain.LevelRule {
		inc.Diagnosis = a.RuleBasedIncidentDiagnosis(inc, graph)
	}
}

// RuleBasedIncidentDiagnosis genera el diagnóstico determinístico del incidente sin IA (I4).
func (a *IncidentAggregator) RuleBasedIncidentDiagnosis(inc *Incident, graph *DependencyGraph) domain.DiagnosisResult {
	originName := inc.EffectiveOrigin()
	var originMetric domain.ContainerMetric
	for id, m := range inc.Members {
		if id == originName || strings.TrimPrefix(m.Name, "/") == originName {
			originMetric = m
			break
		}
	}

	exitCode := domain.ParseExitCode(originMetric.Status)
	ruleRes := a.ruleEngine.Evaluate(exitCode, originMetric.Status, "")
	rootCause := fmt.Sprintf("Cascada por fallo en %s", originName)
	if strings.Contains(strings.ToLower(originMetric.Status), "137") || strings.Contains(strings.ToLower(originMetric.Status), "oom") {
		rootCause = fmt.Sprintf("Cascada por OOM en %s", originName)
	} else if ruleRes != nil && ruleRes.RootCause != "" && ruleRes.RootCause != "Sin diagnóstico concluyente" {
		rootCause = fmt.Sprintf("Cascada por %s en %s", ruleRes.RootCause, originName)
	}

	suggestedAction := "restart"
	if ruleRes != nil && ruleRes.SuggestedAction != "" && ruleRes.SuggestedAction != "none" {
		suggestedAction = ruleRes.SuggestedAction
	}

	return domain.DiagnosisResult{
		Level:           domain.LevelRule,
		RootCause:       rootCause,
		Severity:        "critical",
		SuggestedAction: suggestedAction,
		Confidence:      string(inc.OriginConfidence),
		OriginContainer: originName,
		Cascade:         inc.Cascade,
	}
}

// FormatIncidentBanner formatea la línea del banner para el incidente respetando I7 e I8.
func FormatIncidentBanner(inc *Incident, policy domain.IncidentBannerPolicy, width int, lang ...string) string {
	var tagStyled string
	if inc.Diagnosis.Level == domain.LevelAI {
		tagStyled = "[AI]"
	} else if inc.Diagnosis.Level == domain.LevelAIPartial {
		tagStyled = "[AI~]"
	} else {
		tagStyled = "[RULE]"
	}

	activeLang := i18n.LangES
	if len(lang) > 0 && lang[0] != "" {
		activeLang = i18n.NormalizeLanguage(lang[0])
	}

	group := inc.GroupName
	if group == "" {
		group = i18n.T(activeLang, "incident.default_group")
	}

	// Conteo de cascada: total miembros menos el origen
	affectedCount := max(1, len(inc.Cascade)-1)
	if len(inc.Cascade) <= 1 && len(inc.Members) > 1 {
		// Contar miembros únicos
		seen := make(map[string]bool)
		for _, m := range inc.Members {
			clean := strings.TrimPrefix(m.Name, "/")
			seen[clean] = true
		}
		affectedCount = max(1, len(seen)-1)
	}

	originName := inc.EffectiveOrigin()
	reason := i18n.T(activeLang, "incident.reason_default")
	if ev := findEventFor(inc, originName); ev != nil && ev.Reason != "" {
		reason = ev.Reason
	}

	var content string
	switch inc.OriginConfidence {
	case domain.ConfidenceConfirmed:
		content = i18n.T(activeLang, "incident.confirmed", map[string]interface{}{
			"group":  group,
			"origin": originName,
			"reason": reason,
			"count":  affectedCount,
			"tag":    tagStyled,
		})

	case domain.ConfidenceProbable:
		if policy == domain.BannerPolicyPrudente {
			totalAnom := len(inc.Cascade)
			if totalAnom < 2 {
				totalAnom = countUniqueMembers(inc.Members)
			}
			content = i18n.T(activeLang, "incident.probable_prud", map[string]interface{}{
				"group": group,
				"count": totalAnom,
				"tag":   tagStyled,
			})
		} else {
			// Informativo (default)
			content = i18n.T(activeLang, "incident.probable_info", map[string]interface{}{
				"group":  group,
				"origin": originName,
				"count":  affectedCount,
				"tag":    tagStyled,
			})
		}

	default: // domain.ConfidenceUndetermined ("s/d")
		totalAnom := len(inc.Cascade)
		if totalAnom < 2 {
			totalAnom = countUniqueMembers(inc.Members)
		}
		content = i18n.T(activeLang, "incident.undetermined", map[string]interface{}{
			"group": group,
			"count": totalAnom,
			"tag":   tagStyled,
		})
	}

	return content
}

func countUniqueMembers(m map[string]domain.ContainerMetric) int {
	seen := make(map[string]bool)
	for _, c := range m {
		seen[strings.TrimPrefix(c.Name, "/")] = true
	}
	return len(seen)
}

func findEventFor(inc *Incident, name string) *IncidentEvent {
	clean := strings.TrimPrefix(name, "/")
	for _, ev := range inc.Events {
		if ev.ContainerName == clean {
			return &ev
		}
	}
	return nil
}

// shareHardTopology comprueba si dos contenedores comparten ComposeProject o alguna red (I1).
func shareHardTopology(a, b domain.ContainerMetric) bool {
	if a.ComposeProject != "" && b.ComposeProject != "" && a.ComposeProject == b.ComposeProject {
		return true
	}
	return haveSharedNetwork(a.Networks, b.Networks)
}

// buildHardTopologyComponents encuentra componentes conectados de topología dura entre contenedores anómalos.
func buildHardTopologyComponents(containers []domain.ContainerMetric) [][]domain.ContainerMetric {
	n := len(containers)
	if n == 0 {
		return nil
	}

	visited := make([]bool, n)
	var components [][]domain.ContainerMetric

	for i := 0; i < n; i++ {
		if visited[i] {
			continue
		}
		// BFS / DFS para agrupar
		var comp []domain.ContainerMetric
		queue := []int{i}
		visited[i] = true

		for len(queue) > 0 {
			curr := queue[0]
			queue = queue[1:]
			comp = append(comp, containers[curr])

			for next := 0; next < n; next++ {
				if !visited[next] && shareHardTopology(containers[curr], containers[next]) {
					visited[next] = true
					queue = append(queue, next)
				}
			}
		}

		components = append(components, comp)
	}

	return components
}

// resolveGroupName extrae un nombre representativo para el grupo de topología dura.
func resolveGroupName(comp []domain.ContainerMetric) string {
	if len(comp) == 0 {
		return "incidente"
	}
	// Preferir compose project si coinciden
	proj := comp[0].ComposeProject
	if proj != "" {
		allSame := true
		for _, c := range comp {
			if c.ComposeProject != proj {
				allSame = false
				break
			}
		}
		if allSame {
			return proj
		}
	}

	// Si no, buscar la primera red compartida entre ellos
	if len(comp[0].Networks) > 0 {
		for _, netName := range comp[0].Networks {
			if netName == "" || netName == "bridge" || netName == "host" {
				continue
			}
			allHave := true
			for _, c := range comp[1:] {
				if !containsString(c.Networks, netName) {
					allHave = false
					break
				}
			}
			if allHave {
				return netName
			}
		}
		// Fallback a primera red no vacía
		for _, netName := range comp[0].Networks {
			if netName != "" {
				return netName
			}
		}
	}

	return "incidente"
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func extractAnomalyReason(c domain.ContainerMetric) string {
	exitCode := domain.ParseExitCode(c.Status)
	if exitCode == 137 || strings.Contains(strings.ToLower(c.Status), "oom") {
		return "OOM"
	}
	if exitCode > 0 {
		return fmt.Sprintf("exit %d", exitCode)
	}
	if strings.Contains(strings.ToLower(c.Status), "502") {
		return "502"
	}
	if strings.ToLower(c.Status) == "dead" {
		return c.Status
	}
	return "crash"
}

// CountHealthyPeers cuenta los contenedores del mismo grupo topológico que no presentan anomalías (F4).
func CountHealthyPeers(groupName string, allMetrics []domain.ContainerMetric) int {
	if groupName == "" {
		return 0
	}
	count := 0
	for _, c := range allMetrics {
		if domain.IsAnomalous(c) {
			continue
		}
		if c.ComposeProject == groupName {
			count++
			continue
		}
		for _, net := range c.Networks {
			if net == groupName {
				count++
				break
			}
		}
	}
	return count
}
