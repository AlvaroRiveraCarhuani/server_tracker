# Tareas: TUI de Baja Carga Cognitiva (03-tui-ux-low-cognitive-load)

## Fase 1: Semántica Cromática y Prevención Predictiva OOM
- [x] 1.1 Definir tema canónico Catppuccin Mocha y estilos en `agent/internal/delivery/tui/theme.go`.
- [x] 1.2 Implementar glifos ASCII `[OK]`, `[--]`, `[||]`, `[!!]`.
- [x] 1.3 Implementar alerta predictiva de riesgo OOM al 85% de memoria Working Set.
- [x] 1.4 Implementar semáforo financiero de Egress alineado a la derecha.

## Fase 2: Filtrado Reactivo en Tiempo Real
- [x] 2.1 Integrar `textinput.Model` para activación con tecla `/`.
- [x] 2.2 Implementar lógica de filtrado de lista de contenedores en tiempo real.
- [x] 2.3 Atajos de teclado `Esc` para limpiar filtro y restaurar vista completa.

## Fase 3: Transición Master-Detail (Visor de Logs en Vivo)
- [x] 3.1 Implementar método en `ports.CollectorPort` y `DockerCollector` para streaming de logs de un contenedor (`GetContainerLogs`).
- [x] 3.2 Implementar visor a pantalla completa con `viewport.Model` activable con `l` o `Enter`.
- [x] 3.3 Implementar Header de Breadcrumbs (`[<] Volver (Esc) | Logs: <name> | Estado: [OK]`).

## Fase 4: Blindaje ANSI, Layouts Colapsables y Verificación
- [x] 4.1 Asegurar cálculo de anchos con `lipgloss.Width()` y truncado seguro de nombres.
- [x] 4.2 Validar que todos los tests unitarios y de compilación pasen.
