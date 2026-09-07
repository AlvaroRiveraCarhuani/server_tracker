package domain

import "errors"

// ActionType define las acciones de remediación permitidas (D1).
type ActionType string

const (
	ActionRestart        ActionType = "restart"
	ActionStop           ActionType = "stop"
	ActionIsolateNetwork ActionType = "isolate_network"
)

var (
	ErrUnauthorizedAction = errors.New("acción de remediación no permitida o prohibida (cero RCE)")
	ErrContainerNotFound  = errors.New("contenedor no encontrado")
)

// RemediationCommand representa una orden de control remoto validada.
type RemediationCommand struct {
	HostID      string     `json:"host_id"`
	ContainerID string     `json:"container_id"`
	Action      ActionType `json:"action"`
	Nonce       string     `json:"nonce"`
	Timestamp   int64      `json:"timestamp"`
}

// Validate asegura que la acción esté en la lista blanca estricta (D1).
func (c *RemediationCommand) Validate() error {
	switch c.Action {
	case ActionRestart, ActionStop, ActionIsolateNetwork:
		return nil
	default:
		return ErrUnauthorizedAction
	}
}
