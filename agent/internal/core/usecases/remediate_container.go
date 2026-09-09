package usecases

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
	"github.com/alvaroriverac/server_tracker_agent/internal/core/ports"
)

var (
	ErrNilCollector      = errors.New("collector port is required")
	ErrEmptyContainerID  = errors.New("container ID cannot be empty")
	ErrDisallowedAction  = errors.New("remediation action rejected by zero-rce whitelist policy (D1)")
)

// RemediateContainerUseCase orquesta la ejecución de acciones correctivas con validación de seguridad estricta.
type RemediateContainerUseCase struct {
	collector ports.CollectorPort
}

// NewRemediateContainerUseCase crea una nueva instancia del caso de uso.
func NewRemediateContainerUseCase(collector ports.CollectorPort) *RemediateContainerUseCase {
	return &RemediateContainerUseCase{
		collector: collector,
	}
}

// Execute valida que la acción pertenezca a la lista blanca permitida y la ejecuta a través del adaptador de Docker.
func (uc *RemediateContainerUseCase) Execute(ctx context.Context, cmd domain.RemediationCommand) error {
	if uc.collector == nil {
		return ErrNilCollector
	}
	if strings.TrimSpace(cmd.ContainerID) == "" {
		return ErrEmptyContainerID
	}

	// Validación estricta D1: Cero RCE
	switch cmd.Action {
	case domain.ActionRestart, domain.ActionStop, domain.ActionIsolateNetwork:
		// Acción autorizada en la whitelist
	default:
		return fmt.Errorf("%w: acción '%s' no permitida", ErrDisallowedAction, cmd.Action)
	}

	return uc.collector.ExecuteRemediation(ctx, cmd)
}
