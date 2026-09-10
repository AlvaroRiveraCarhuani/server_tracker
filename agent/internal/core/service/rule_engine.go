package service

import (
	"strings"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
)

// RuleEngine evalúa incidentes de contenedores mediante reglas determinísticas locales (costo cero, sin IA).
type RuleEngine struct {
	rules []domain.LocalRule
}

// NewRuleEngine inicializa el motor con una lista de reglas o con las reglas por defecto si no se especifican.
func NewRuleEngine(customRules ...[]domain.LocalRule) *RuleEngine {
	rules := domain.DefaultLocalRules
	if len(customRules) > 0 && customRules[0] != nil {
		rules = customRules[0]
	}
	return &RuleEngine{
		rules: rules,
	}
}

// Evaluate busca una regla coincidente por ExitCode, patrón en logs o estado del contenedor.
// Retorna un DiagnosisResult con Level = domain.LevelRule si hay coincidencia, o nil si cae a señal pura (Nivel 3).
func (re *RuleEngine) Evaluate(exitCode int, status string, lastLogLines string) *domain.DiagnosisResult {
	statusLower := strings.ToLower(strings.TrimSpace(status))
	logsLower := strings.ToLower(lastLogLines)

	for _, rule := range re.rules {
		// 1. Coincidencia por estado explícito (ej: "CrashLoopBackOff", "OOMKilled")
		if rule.MatchStatus != "" {
			if strings.Contains(statusLower, strings.ToLower(rule.MatchStatus)) {
				return &domain.DiagnosisResult{
					Level:           domain.LevelRule,
					RootCause:       rule.RootCause,
					Severity:        rule.Severity,
					SuggestedAction: rule.SuggestedAction,
				}
			}
		}

		// 2. Coincidencia por ExitCode
		if rule.MatchExitCode != nil && *rule.MatchExitCode == exitCode {
			// Si la regla requiere coincidencia en logs
			if rule.MatchLogPattern != "" {
				if strings.Contains(logsLower, strings.ToLower(rule.MatchLogPattern)) {
					return &domain.DiagnosisResult{
						Level:           domain.LevelRule,
						RootCause:       rule.RootCause,
						Severity:        rule.Severity,
						SuggestedAction: rule.SuggestedAction,
					}
				}
				// Si no coincide el patrón de log, continuamos evaluando otras reglas
				continue
			}

			// Regla de exit code sin requisito de logs
			return &domain.DiagnosisResult{
				Level:           domain.LevelRule,
				RootCause:       rule.RootCause,
				Severity:        rule.Severity,
				SuggestedAction: rule.SuggestedAction,
			}
		}
	}

	return nil
}
