# Tareas: Sparklines y Tendencias en la TUI (07-tui-sparklines-history)

## Fase 1: Módulo de Sparklines y Buffer de Tendencias
- [x] 1.1 Implementar generador puro de sparklines en `agent/internal/delivery/tui/sparkline.go`.
- [x] 1.2 Implementar gestión de buffer acotado (máx 30 muestras) con poda de contenedores extintos.
- [x] 1.3 Crear tests unitarios en `agent/internal/delivery/tui/sparkline_test.go`.

## Fase 2: Integración en la TUI
- [x] 2.1 Conectar el recolector de métricas al buffer de tendencias en `tui.go`.
- [x] 2.2 Incorporar columna condicional `CPU TREND` en la tabla principal `viewTable()`.
- [x] 2.3 Incorporar panel de tendencias en el encabezado de `viewLogs()`.

## Fase 3: Validación y Verificación
- [x] 3.1 Ejecutar conjunto de tests con `-race` y validar que todos pasen en verde.
- [x] 3.2 Compilar binario de `solv-agent`.
- [x] 3.3 Actualizar `docs/ROADMAP.md` y comitear cambios.

