# Especificación: Remediación en Caliente por Teclado en la TUI (tui-hot-remediation)

## Requisitos

### R1: Atajos de Teclado de Remediación
- Cuando la TUI se encuentra en `stateFleetTable`, las siguientes teclas activan el modal de confirmación sobre el contenedor apuntado por el cursor:
  - `r`: Prepara acción `ActionRestart`.
  - `s`: Prepara acción `ActionStop`.
  - `x`: Prepara acción `ActionIsolateNetwork`.

### R2: Modal de Confirmación Segura
- El modal debe mostrar el nombre del contenedor, su ID corto y la acción a realizar.
- Solo las teclas `y` o `Enter` confirman la ejecución.
- Las teclas `n`, `Esc` o `q` cancelan inmediatamente la acción sin alterar el contenedor.

### R3: Ejecución Asíncrona sin Bloqueo
- La ejecución en el Docker daemon debe realizarse de forma asíncrona mediante un `tea.Cmd`.
- La interfaz debe permanecer interactiva mientras se ejecuta la orden.
- Al finalizar, debe mostrarse el resultado con el tiempo transcurrido en la barra de estado.
