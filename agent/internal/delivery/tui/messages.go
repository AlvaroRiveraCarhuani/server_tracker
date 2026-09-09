package tui

import (
	"context"
	"time"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
	tea "github.com/charmbracelet/bubbletea"
)

type sessionState int

const (
	stateFleetTable sessionState = iota
	stateLogViewer
	stateFiltering
	stateConfirmRemediation
	stateConfigModal
	stateHelp
)

type tickMsg time.Time

type logsMsg struct {
	containerName string
	content       string
	err           error
}

type remediationResultMsg struct {
	action        domain.ActionType
	containerName string
	elapsed       time.Duration
	err           error
}

type diagnosisResultMsg struct {
	containerID string
	diagnosis   string
	usage       domain.TokenUsage
}

type shellFinishedMsg struct {
	err error
}

func tickCmd() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// TriageService define el contrato para el análisis pasivo con IA y reporte de tokens.
type TriageService interface {
	DiagnoseContainer(ctx context.Context, name, image, status, logs string) string
	DiagnoseContainerWithUsage(ctx context.Context, name, image, status, logs string) (string, domain.TokenUsage)
}
