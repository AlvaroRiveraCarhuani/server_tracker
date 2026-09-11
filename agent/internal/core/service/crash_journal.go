package service

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
)

const maxEventsPerContainer = 50

// CrashEvent representa un evento individual de crash o reinicio anómalo de un contenedor.
type CrashEvent struct {
	Timestamp time.Time
	ExitCode  int
	IsOOM     bool
	Key       string // "oom" si isOOM o exit 137; si no "exit-<code>"
	Diagnosis string // resumen del diagnóstico emitido (root cause)
	Level     string // nivel de procedencia (AI/AI~/RULE/SIG)
}

// CrashJournal gestiona la memoria temporal volátil de incidentes por contenedor en memoria (H2).
type CrashJournal struct {
	mu           sync.RWMutex
	events       map[string][]CrashEvent // containerID y name -> ring buffer de hasta 50 eventos
	recordedKeys map[string]bool         // dedupe key -> bool
	aliasMap     map[string]string       // name -> ID
}

// NewCrashJournal inicializa una instancia limpia y volátil de CrashJournal.
func NewCrashJournal() *CrashJournal {
	return &CrashJournal{
		events:       make(map[string][]CrashEvent),
		recordedKeys: make(map[string]bool),
		aliasMap:     make(map[string]string),
	}
}

// Record registra un incidente en el journal deduplicando por (containerID, finishedAt, exitCode).
// Retorna true si fue registrado como nuevo evento, o false si fue ignorado por dedupe.
func (j *CrashJournal) Record(c domain.ContainerMetric, diag string, level string) (bool, string) {
	j.mu.Lock()
	defer j.mu.Unlock()

	exitCode := domain.ParseExitCode(c.Status)
	isOOM := exitCode == 137 || strings.Contains(strings.ToLower(c.Status), "oom")

	key := fmt.Sprintf("exit-%d", exitCode)
	if isOOM || exitCode == 137 {
		key = "oom"
	}

	finishedAt := c.LastStateChange
	var dedupeKey string
	if !finishedAt.IsZero() {
		dedupeKey = fmt.Sprintf("%s:%d:%d", c.ID, finishedAt.UnixNano(), exitCode)
	} else {
		dedupeKey = fmt.Sprintf("%s:%s:%d", c.ID, c.Status, exitCode)
	}

	if j.recordedKeys[dedupeKey] {
		return false, key
	}
	j.recordedKeys[dedupeKey] = true

	now := time.Now()
	ev := CrashEvent{
		Timestamp: now,
		ExitCode:  exitCode,
		IsOOM:     isOOM,
		Key:       key,
		Diagnosis: diag,
		Level:     level,
	}

	// Asociar alias de nombre a ID
	if c.Name != "" && c.ID != "" {
		j.aliasMap[c.Name] = c.ID
	}

	id := c.ID
	if id == "" {
		id = c.Name
	}

	list := j.events[id]
	list = append(list, ev)
	if len(list) > maxEventsPerContainer {
		list = list[len(list)-maxEventsPerContainer:]
	}
	j.events[id] = list

	return true, key
}

// UpdateDiagnosis actualiza la causa raíz y nivel del último evento registrado de un contenedor.
func (j *CrashJournal) UpdateDiagnosis(containerRef string, diag string, level string) {
	j.mu.Lock()
	defer j.mu.Unlock()

	id := j.resolveID(containerRef)
	list := j.events[id]
	if len(list) > 0 {
		list[len(list)-1].Diagnosis = diag
		list[len(list)-1].Level = level
		j.events[id] = list
	}
}

// Count cuenta los eventos dentro de una ventana de tiempo que coincidan con key (o todos si key es vacío).
func (j *CrashJournal) Count(containerRef string, key string, window time.Duration) int {
	j.mu.RLock()
	defer j.mu.RUnlock()

	id := j.resolveID(containerRef)
	list := j.events[id]
	if len(list) == 0 {
		return 0
	}

	cutoff := time.Now().Add(-window)
	count := 0
	for _, ev := range list {
		if ev.Timestamp.After(cutoff) {
			if key == "" || ev.Key == key {
				count++
			}
		}
	}
	return count
}

// LastEvent retorna el evento más reciente del contenedor o nil si no hay eventos.
func (j *CrashJournal) LastEvent(containerRef string) *CrashEvent {
	j.mu.RLock()
	defer j.mu.RUnlock()

	id := j.resolveID(containerRef)
	list := j.events[id]
	if len(list) == 0 {
		return nil
	}
	last := list[len(list)-1]
	return &last
}

// PreviousDiagnosis retorna el diagnóstico emitido previo al evento actual (si existe).
func (j *CrashJournal) PreviousDiagnosis(containerRef string) string {
	j.mu.RLock()
	defer j.mu.RUnlock()

	id := j.resolveID(containerRef)
	list := j.events[id]
	if len(list) < 2 {
		return ""
	}
	// Buscar el diagnóstico más reciente anterior al último
	for i := len(list) - 2; i >= 0; i-- {
		if strings.TrimSpace(list[i].Diagnosis) != "" {
			return list[i].Diagnosis
		}
	}
	return ""
}

// Homogeneity analiza si todos los eventos dentro de la ventana comparten key ("todos OOM", "todos exit-1" o "mixtos").
func (j *CrashJournal) Homogeneity(containerRef string, window time.Duration) string {
	j.mu.RLock()
	defer j.mu.RUnlock()

	id := j.resolveID(containerRef)
	list := j.events[id]
	if len(list) == 0 {
		return ""
	}

	cutoff := time.Now().Add(-window)
	var matchingKeys []string
	for _, ev := range list {
		if ev.Timestamp.After(cutoff) {
			matchingKeys = append(matchingKeys, ev.Key)
		}
	}

	if len(matchingKeys) == 0 {
		return ""
	}

	first := matchingKeys[0]
	for _, k := range matchingKeys[1:] {
		if k != first {
			return "mixtos"
		}
	}

	if first == "oom" {
		return "todos OOM"
	}
	return "todos " + first
}

// IsRecurrent determina si se cumple el umbral estricto H3 (>= 3 eventos con la misma key en 1 hora).
func (j *CrashJournal) IsRecurrent(containerRef string, key string) (bool, int) {
	count := j.Count(containerRef, key, 1*time.Hour)
	return count >= 3, count
}

// FormatPromptBlock construye el bloque 'Historial reciente' y la instrucción de guardia para el prompt de IA.
// Retorna "" si el contenedor no tiene al menos un evento previo en el journal.
func (j *CrashJournal) FormatPromptBlock(containerRef string, ramTrend string) string {
	j.mu.RLock()
	defer j.mu.RUnlock()

	id := j.resolveID(containerRef)
	list := j.events[id]
	if len(list) == 0 {
		return ""
	}

	count1h := 0
	cutoff := time.Now().Add(-1 * time.Hour)
	var windowKeys []string
	var lastEv *CrashEvent

	for i := range list {
		ev := list[i]
		if ev.Timestamp.After(cutoff) {
			count1h++
			windowKeys = append(windowKeys, ev.Key)
		}
		lastEv = &ev
	}

	if count1h == 0 && len(list) > 0 {
		// Si no hay en 1h pero hay previos históricos
		count1h = len(list)
	}

	homog := "todos " + list[len(list)-1].Key
	if list[len(list)-1].Key == "oom" {
		homog = "todos OOM"
	}
	if len(windowKeys) > 1 {
		first := windowKeys[0]
		for _, k := range windowKeys[1:] {
			if k != first {
				homog = "mixtos"
				break
			}
		}
	}

	lastAgo := "reciente"
	if lastEv != nil {
		d := time.Since(lastEv.Timestamp)
		if d < time.Minute {
			lastAgo = fmt.Sprintf("hace %ds", max(1, int(d.Seconds())))
		} else if d < time.Hour {
			lastAgo = fmt.Sprintf("hace %dm", max(1, int(d.Minutes())))
		} else {
			lastAgo = fmt.Sprintf("hace %dh", max(1, int(d.Hours())))
		}
	}

	var lines []string
	lines = append(lines, "Historial reciente:")
	lines = append(lines, fmt.Sprintf("  crashes: %d en la última hora (%s) · último %s", count1h, homog, lastAgo))

	// Hipótesis previa (si existe)
	prevDiag := ""
	for i := len(list) - 1; i >= 0; i-- {
		if strings.TrimSpace(list[i].Diagnosis) != "" {
			prevDiag = list[i].Diagnosis
			break
		}
	}
	if prevDiag != "" {
		lines = append(lines, fmt.Sprintf("  hipótesis previa: %s", prevDiag))
	}

	// Tendencia RAM (si existe)
	if strings.TrimSpace(ramTrend) != "" {
		lines = append(lines, fmt.Sprintf("  tendencia RAM: %s", ramTrend))
	}

	lines = append(lines, "")
	lines = append(lines, "INSTRUCCIÓN: la hipótesis previa es hipótesis, no verdad. Verifícala contra la evidencia nueva. Descártala si la contradice.")

	return strings.Join(lines, "\n")
}

func (j *CrashJournal) resolveID(ref string) string {
	if id, ok := j.aliasMap[ref]; ok {
		return id
	}
	return ref
}
