package usecases

import (
	"context"
	"errors"
	"strings"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
	"github.com/alvaroriverac/server_tracker_agent/internal/core/ports"
)

var (
	ErrNilTriageClient = errors.New("triage port is required")
)

// DiagnoseContainerUseCase orquesta la recolección de contexto y el triaje con IA para contenedores anómalos.
type DiagnoseContainerUseCase struct {
	collector  ports.CollectorPort
	triagePort ports.TriagePort
}

// NewDiagnoseContainerUseCase crea una nueva instancia del caso de uso.
func NewDiagnoseContainerUseCase(collector ports.CollectorPort, triage ports.TriagePort) *DiagnoseContainerUseCase {
	return &DiagnoseContainerUseCase{
		collector:  collector,
		triagePort: triage,
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
