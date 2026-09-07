# Propuesta: Remediación en Caliente por Teclado en la TUI (05-tui-hot-remediation)

## Intención

Transformar la TUI de SOLV Server Tracker de una consola de solo lectura a un centro de mando operativo activo. El operador podrá reiniciar, detener o aislar contenedores directamente desde su teclado con atajos (`r`, `s`, `x`), protegidos por un diálogo modal de confirmación segura con Lip Gloss y retroalimentación en tiempo real en la barra de estado.

## Alcance

### Dentro del Alcance
- **Atajos de Remediación en la TUI**:
  - `r`: Reiniciar contenedor seleccionado (`ActionRestart`).
  - `s`: Detener contenedor seleccionado (`ActionStop`).
  - `x`: Aislar contenedor de la red (`ActionIsolateNetwork`, desconectando endpoints de red para cortar picos de egress o contener incidentes).
- **Modal de Confirmación Segura**:
  - Estado `stateConfirmRemediation` con diálogo flotante y bordes redondeados en la paleta Catppuccin Mocha.
  - Prevención de disparos accidentales: requiere confirmación explícita con `y` o `Enter`, o cancelación con `n` o `Esc`.
- **Ejecución Asíncrona sin Congelar la UI**:
  - Llamada a `ports.CollectorPort.ExecuteRemediation` mediante `tea.Cmd`.
  - Animación de estado en progreso en la barra inferior.
  - Notificación de resultado con tiempo de ejecución (`[OK] Reiniciado en 780ms`).
- **Tests Automatizados**:
  - Tests en `agent/internal/delivery/tui/` validando transiciones de estado, confirmación, cancelación y despacho de la remediación.

### Fuera del Alcance
- Ejecución de comandos de shell arbitrarios (`docker exec` prohibido por D1).
- Eliminación forzada destructiva de volúmenes o imágenes.

## Capacidades

### Nuevas Capacidades
- `tui-hot-remediation`: Atajos directos de remediación interactiva protegida con modal de confirmación y feedback visual.
