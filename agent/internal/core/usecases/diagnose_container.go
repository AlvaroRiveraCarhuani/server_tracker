package usecases

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
	"github.com/alvaroriverac/server_tracker_agent/internal/core/ports"
	"github.com/alvaroriverac/server_tracker_agent/internal/core/service"
)

var (
	ErrNilTriageClient = errors.New("triage port is required")
)

// DiagnoseContainerUseCase orquesta la recolección de contexto, el triaje con IA y el motor de reglas locales.
type DiagnoseContainerUseCase struct {
	collector    ports.CollectorPort
	triagePort   ports.TriagePort
	ruleEngine   *service.RuleEngine
	crashJournal *service.CrashJournal
	parser       *service.AIResponseParser
	mu           sync.Mutex
	reExecuted   map[string]bool
}

// NewDiagnoseContainerUseCase crea una nueva instancia del caso de uso.
func NewDiagnoseContainerUseCase(collector ports.CollectorPort, triage ports.TriagePort, ruleEngines ...*service.RuleEngine) *DiagnoseContainerUseCase {
	var re *service.RuleEngine
	if len(ruleEngines) > 0 && ruleEngines[0] != nil {
		re = ruleEngines[0]
	} else {
		re = service.NewRuleEngine()
	}
	return &DiagnoseContainerUseCase{
		collector:    collector,
		triagePort:   triage,
		ruleEngine:   re,
		parser:       service.NewAIResponseParser(),
		reExecuted:   make(map[string]bool),
	}
}

// SetCrashJournal vincula el journal de fallas para contexto temporal y detección de recurrencia.
func (uc *DiagnoseContainerUseCase) SetCrashJournal(journal *service.CrashJournal) {
	uc.crashJournal = journal
}

// SetReExecutedMap permite inyectar el mapa de eventos re-ejecutados para dedupe en sesión.
func (uc *DiagnoseContainerUseCase) SetReExecutedMap(reExecuted map[string]bool) {
	if reExecuted != nil {
		uc.reExecuted = reExecuted
	}
}

// Execute recopila logs recientes si están disponibles y solicita un diagnóstico contextual de causa raíz con IA.
func (uc *DiagnoseContainerUseCase) Execute(ctx context.Context, c domain.ContainerMetric) (string, domain.TokenUsage, error) {
	if uc.triagePort == nil {
		return "", domain.TokenUsage{}, ErrNilTriageClient
	}

	var logs string
	if uc.collector != nil && c.ID != "" {
		fetchedLogs, err := uc.collector.GetContainerLogs(ctx, c.ID, 50)
		if err == nil {
			logs = strings.TrimSpace(fetchedLogs)
		}
	}

	diag, usage := uc.triagePort.DiagnoseContainerWithUsage(ctx, c.Name, c.Image, c.Status, logs)
	return diag, usage, nil
}

// ExecuteWithCascade ejecuta la cascada de 4 niveles de diagnóstico:
// Nivel 0 [AI]: IA disponible y respuesta formateada.
// Nivel 1 [AI~]: IA respondió pero fallo de formato/parse (raw truncado a 1 línea).
// Nivel 2 [RULE]: Fallo de IA o modo MANUAL sin trigger explícito -> Motor de reglas locales.
// Nivel 3 [SIG]: Señal cruda sin regla coincidente.
func (uc *DiagnoseContainerUseCase) ExecuteWithCascade(
	ctx context.Context,
	c domain.ContainerMetric,
	forceAI bool,
	selectionMode domain.ModelSelectionMode,
) domain.DiagnosisResult {
	res := uc.executeCascadeRaw(ctx, c, forceAI, selectionMode)
	return uc.enrichWithRecurrence(c, res)
}

func (uc *DiagnoseContainerUseCase) executeCascadeRaw(
	ctx context.Context,
	c domain.ContainerMetric,
	forceAI bool,
	selectionMode domain.ModelSelectionMode,
) domain.DiagnosisResult {
	// 1. Obtener logs recientes
	var logs string
	if uc.collector != nil && c.ID != "" {
		if fetchedLogs, err := uc.collector.GetContainerLogs(ctx, c.ID, 50); err == nil {
			logs = strings.TrimSpace(fetchedLogs)
		}
	}

	exitCode := parseExitCode(c.Status)

	// 2. Si el modo es MANUAL y no se forzó IA explícitamente con la tecla 'i', saltar directo a reglas locales
	if selectionMode == domain.SelectionManual && !forceAI {
		if localRes := uc.ruleEngine.Evaluate(exitCode, c.Status, logs); localRes != nil {
			return *localRes
		}
		return signalResult(exitCode, c.Status)
	}

	// 3. Evaluar con IA si el puerto está configurado
	if uc.triagePort != nil {
		rawDiag, usage := uc.triagePort.DiagnoseContainerWithSlot(ctx, c.Name, c.Image, c.Status, logs, domain.SlotFast)
		rawDiag = strings.TrimSpace(rawDiag)

		// Comprobar si hubo fallo operativo en IA (sin key, timeout, error HTTP o no configurado)
		if isAIFailure(rawDiag) {
			// Degradar a reglas locales
			if localRes := uc.ruleEngine.Evaluate(exitCode, c.Status, logs); localRes != nil {
				return *localRes
			}
			return signalResult(exitCode, c.Status)
		}

		// IA respondió con éxito. Parsear con AIResponseParser (S1)
		res := uc.parser.Parse(rawDiag, usage, c.Status)

		// S2: Re-ejecución acotada FAST -> DEEP si severity == critical && confidence != low
		if res.Level == domain.LevelAI && res.Severity == "critical" && res.Confidence != "low" {
			eventKey := fmt.Sprintf("%s:%d:%d", c.ID, c.LastStateChange.UnixNano(), exitCode)
			if c.LastStateChange.IsZero() {
				eventKey = fmt.Sprintf("%s:%s:%d", c.ID, c.Status, exitCode)
			}

			uc.mu.Lock()
			already := uc.reExecuted[eventKey]
			if !already {
				uc.reExecuted[eventKey] = true
			}
			uc.mu.Unlock()

			if !already {
				deepDiag, deepUsage := uc.triagePort.DiagnoseContainerWithSlot(ctx, c.Name, c.Image, c.Status, logs, domain.SlotDeep)
				deepDiag = strings.TrimSpace(deepDiag)
				if !isAIFailure(deepDiag) {
					deepRes := uc.parser.Parse(deepDiag, deepUsage, c.Status)
					if deepRes.Level == domain.LevelAI {
						res = deepRes
						res.ReanalyzedDeep = true
						res.ProcessNote = "re-analizado en DEEP por severidad crítica"
					}
				}
			}
		}

		return res
	}

	// 4. Si no hay triagePort, evaluar con reglas locales
	if localRes := uc.ruleEngine.Evaluate(exitCode, c.Status, logs); localRes != nil {
		return *localRes
	}

	return signalResult(exitCode, c.Status)
}

func (uc *DiagnoseContainerUseCase) enrichWithRecurrence(c domain.ContainerMetric, res domain.DiagnosisResult) domain.DiagnosisResult {
	if uc.crashJournal == nil {
		return res
	}

	exitCode := parseExitCode(c.Status)
	isOOM := exitCode == 137 || strings.Contains(strings.ToLower(c.Status), "oom")
	key := fmt.Sprintf("exit-%d", exitCode)
	if isOOM || exitCode == 137 {
		key = "oom"
	}

	uc.crashJournal.Record(c, res.RootCause, string(res.Level))

	isRec, count := uc.crashJournal.IsRecurrent(c.ID, key)
	if !isRec && c.Name != "" {
		isRec, count = uc.crashJournal.IsRecurrent(c.Name, key)
	}

	if isRec {
		res.RecurrenceCount = count
		res.RecurrenceNote = "reiniciar no resolverá la causa raíz"
		res.RootCause = fmt.Sprintf("%s recurrente (%d/1h)", res.RootCause, count)
	}

	uc.crashJournal.UpdateDiagnosis(c.ID, res.RootCause, string(res.Level))
	return res
}

// isAIFailure detecta si la respuesta del cliente de IA indica un error o indisponibilidad del servicio de IA.
func isAIFailure(resp string) bool {
	if resp == "" {
		return true
	}
	rLower := strings.ToLower(resp)
	return strings.HasPrefix(rLower, "error al") ||
		strings.HasPrefix(rLower, "error conectando") ||
		strings.HasPrefix(rLower, "error preparando") ||
		strings.HasPrefix(rLower, "error leyendo") ||
		strings.HasPrefix(rLower, "error serializando") ||
		strings.HasPrefix(rLower, "error http") ||
		strings.Contains(rLower, "diagnóstico no configurado") ||
		strings.Contains(rLower, "diagnóstico no disponible") ||
		strings.Contains(rLower, "no disponible") ||
		strings.Contains(rLower, "tiempo de espera agotado") ||
		strings.Contains(rLower, "cuota saturada") ||
		strings.Contains(rLower, "ia no disponible") ||
		strings.Contains(rLower, "respuesta no concluyente") ||
		strings.Contains(rLower, "401 unauthorized") ||
		strings.Contains(rLower, "429 too many requests")
}


// signalResult construye el diagnóstico Nivel 3 [SIG].
func signalResult(exitCode int, status string) domain.DiagnosisResult {
	cause := "Sin diagnóstico"
	if exitCode >= 0 {
		cause = fmt.Sprintf("Exit code %d · sin diagnóstico", exitCode)
	} else if status != "" {
		cause = fmt.Sprintf("%s · sin diagnóstico", status)
	}

	return domain.DiagnosisResult{
		Level:           domain.LevelSignal,
		RootCause:       cause,
		Severity:        "info",
		SuggestedAction: "none",
	}
}

// parseExitCode delega en domain.ParseExitCode para extracción canónica.
func parseExitCode(status string) int {
	return domain.ParseExitCode(status)
}

