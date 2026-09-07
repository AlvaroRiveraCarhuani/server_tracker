# Tareas: Remediación en Caliente por Teclado en la TUI (05-tui-hot-remediation)

## Fase 1: Extensión de la Máquina de Estados de la TUI
- [x] 1.1 Agregar estado `stateConfirmRemediation` y campos de acción pendiente en `agent/internal/delivery/tui/tui.go`.
- [x] 1.2 Implementar captura de teclas `r` (restart), `s` (stop), `x` (isolate_network).
- [x] 1.3 Implementar diálogo modal de confirmación segura con Lip Gloss.

## Fase 2: Ejecución Asíncrona de Remediación
- [x] 2.1 Implementar `executeRemediationCmd` que invoca `m.collector.ExecuteRemediation` con timeout sin congelar la UI.
- [x] 2.2 Manejar mensaje de resultado `remediationResultMsg` con feedback visual (`[OK]` o `[!!]`).
- [x] 2.3 Refrescar automáticamente la lista de contenedores tras la ejecución de la remediación.

## Fase 3: Tests Automatizados y Verificación
- [x] 3.1 Escribir tests unitarios para la máquina de estados y remediación en `agent/internal/delivery/tui/tui_test.go`.
- [x] 3.2 Validar que la compilación y toda la batería de pruebas pasen en verde.

