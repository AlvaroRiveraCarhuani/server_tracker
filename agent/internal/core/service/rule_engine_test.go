package service

import (
	"testing"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
)

func TestRuleEngine_Evaluate(t *testing.T) {
	re := NewRuleEngine()

	tests := []struct {
		name            string
		exitCode        int
		status          string
		logs            string
		expectedMatch   bool
		expectedCause   string
		expectedAction  string
		expectedSev     string
	}{
		{
			name:           "Exit 137 OOMKilled",
			exitCode:       137,
			status:         "Exited (137) 1 minute ago",
			logs:           "",
			expectedMatch:  true,
			expectedCause:  "OOMKilled: contenedor superó límite de memoria",
			expectedAction: "restart",
			expectedSev:    "critical",
		},
		{
			name:           "Exit 143 SIGTERM",
			exitCode:       143,
			status:         "Exited (143) 2 minutes ago",
			logs:           "",
			expectedMatch:  true,
			expectedCause:  "Terminado por SIGTERM/timeout",
			expectedAction: "none",
			expectedSev:    "warning",
		},
		{
			name:           "Exit 1 connection refused",
			exitCode:       1,
			status:         "Exited (1) 10 seconds ago",
			logs:           "ERROR: dial tcp 127.0.0.1:5432: Connection Refused in pool",
			expectedMatch:  true,
			expectedCause:  "Servicio dependiente no alcanzable",
			expectedAction: "none",
			expectedSev:    "critical",
		},
		{
			name:           "Exit 1 address already in use",
			exitCode:       1,
			status:         "Exited (1) 5 seconds ago",
			logs:           "FATAL: listen tcp :8080: bind: address already in use",
			expectedMatch:  true,
			expectedCause:  "Conflicto de puerto en el host",
			expectedAction: "stop",
			expectedSev:    "critical",
		},
		{
			name:           "Exit 1 no space left on device",
			exitCode:       1,
			status:         "Exited (1) 30 seconds ago",
			logs:           "write /var/log/app.log: No space left on device",
			expectedMatch:  true,
			expectedCause:  "Sin espacio en disco",
			expectedAction: "none",
			expectedSev:    "critical",
		},
		{
			name:           "Exit 1 permission denied",
			exitCode:       1,
			status:         "Exited (1) 40 seconds ago",
			logs:           "open /data/db: Permission Denied",
			expectedMatch:  true,
			expectedCause:  "Error de permisos en volúmenes/archivos",
			expectedAction: "none",
			expectedSev:    "warning",
		},
		{
			name:           "Exit 1 exec format error",
			exitCode:       1,
			status:         "Exited (1) 1 second ago",
			logs:           "standard_init_linux.go: exec format error",
			expectedMatch:  true,
			expectedCause:  "Mismatch de arquitectura (ej: amd64 en arm64)",
			expectedAction: "none",
			expectedSev:    "critical",
		},
		{
			name:           "Exit 1 kernel killed",
			exitCode:       1,
			status:         "Exited (1) 12 seconds ago",
			logs:           "Killed process 1024 (node)",
			expectedMatch:  true,
			expectedCause:  "Proceso eliminado por el kernel (posible OOM global)",
			expectedAction: "none",
			expectedSev:    "critical",
		},
		{
			name:           "Status CrashLoopBackOff",
			exitCode:       1,
			status:         "CrashLoopBackOff (5 retries)",
			logs:           "Starting...",
			expectedMatch:  true,
			expectedCause:  "Fallo recurrente en el arranque",
			expectedAction: "restart",
			expectedSev:    "critical",
		},
		{
			name:           "Status OOMKilled",
			exitCode:       0,
			status:         "OOMKilled by cgroup",
			logs:           "",
			expectedMatch:  true,
			expectedCause:  "OOMKilled: límite de memoria excedido",
			expectedAction: "restart",
			expectedSev:    "critical",
		},
		{
			name:           "Exit 255 invalid config",
			exitCode:       255,
			status:         "Exited (255) 1 hour ago",
			logs:           "",
			expectedMatch:  true,
			expectedCause:  "Error de configuración o comando de entrada inválido",
			expectedAction: "none",
			expectedSev:    "warning",
		},
		{
			name:           "Exit 0 unexpected restart",
			exitCode:       0,
			status:         "Exited (0) 5 seconds ago",
			logs:           "Exited cleanly",
			expectedMatch:  true,
			expectedCause:  "Reinicio limpio pero inesperado",
			expectedAction: "none",
			expectedSev:    "info",
		},
		{
			name:          "Exit 42 unknown error without matching rule",
			exitCode:      42,
			status:        "Exited (42)",
			logs:          "custom error trace",
			expectedMatch: false,
		},
		{
			name:          "Exit 1 with unmatched log line",
			exitCode:      1,
			status:        "Exited (1)",
			logs:          "general failure without known keywords",
			expectedMatch: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := re.Evaluate(tc.exitCode, tc.status, tc.logs)
			if !tc.expectedMatch {
				if res != nil {
					t.Fatalf("expected nil result for %s, got %+v", tc.name, res)
				}
				return
			}

			if res == nil {
				t.Fatalf("expected match for %s, got nil", tc.name)
			}
			if res.Level != domain.LevelRule {
				t.Errorf("expected LevelRule, got %v", res.Level)
			}
			if res.RootCause != tc.expectedCause {
				t.Errorf("expected RootCause %q, got %q", tc.expectedCause, res.RootCause)
			}
			if res.SuggestedAction != tc.expectedAction {
				t.Errorf("expected SuggestedAction %q, got %q", tc.expectedAction, res.SuggestedAction)
			}
			if res.Severity != tc.expectedSev {
				t.Errorf("expected Severity %q, got %q", tc.expectedSev, res.Severity)
			}
		})
	}
}
