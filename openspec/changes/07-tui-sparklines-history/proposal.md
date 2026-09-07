# Propuesta: Sparklines y Tendencias de CPU/RAM en Tiempo Real en la TUI (07-tui-sparklines-history)

## Motivación
En una consola de observabilidad para Docker en terminal, un valor numérico puntual (como `CPU: 15%`) no le dice al operador si el contenedor viene de un pico agresivo, si está estabilizado o si experimenta ciclos continuos de throttling. Incorporar sparklines ASCII de alta densidad informativa permite evaluar la salud temporal de un solo vistazo sin salir de la TUI ni requerir herramientas web pesadas.

## Alcance
1. **Módulo de Historial en Memoria**:
   - Estructura `MetricHistory` con buffers acotados (últimas 30 muestras) para CPU y RAM por contenedor.
   - Poda automática de contenedores que ya no existen en Docker para evitar consumo residual de memoria.
2. **Generador de Sparklines ASCII**:
   - Función pura y testeada `RenderSparkline(values []float64, minVal, maxVal float64) string` usando bloques unicode (` ▂▃▅▆▇█`).
   - Semántica cromática con Catppuccin Mocha (Green para rango normal, Peach para advertencia y Red para picos críticos).
3. **Renderizado en la TUI**:
   - Columna `CPU TREND` en la tabla de contenedores para terminales con ancho suficiente.
   - Resumen temporal de CPU y Working Set RAM en el encabezado del visor de detalle/logs.
