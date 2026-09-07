# Tareas: Diagnóstico Pasivo Zero-Prompt con OpenRouter en la TUI (06-tui-zero-prompt-ai)

## Fase 1: Cliente Ligero de Diagnóstico OpenRouter en Go
- [x] 1.1 Implementar cliente de inferencia en `agent/internal/infrastructure/ai/triage.go` con `http.Client` y timeout de 6s.
- [x] 1.2 Agregar puerto o método para consulta de diagnóstico en el paquete delivery/tui.

## Fase 2: Integración Reactiva en el Modelo de la TUI
- [x] 2.1 Agregar `diagnosisCache map[string]string` y mensaje `diagnosisResultMsg` en `tui.go`.
- [x] 2.2 Disparar la consulta en segundo plano al mover el cursor sobre un contenedor con estado `STOPPED`, `OOM_RISK` o fallo.
- [x] 2.3 Renderizar el banner contextual AIOps en `viewTable()`.

## Fase 3: Tests Automatizados y Verificación
- [x] 3.1 Escribir tests unitarios en `agent/internal/infrastructure/ai/triage_test.go` y `agent/internal/delivery/tui/tui_test.go`.
- [x] 3.2 Validar que todos los tests pasen en verde.

