# Propuesta: Rediseño de TUI de Baja Carga Cognitiva y Alta Densidad (03-tui-ux-low-cognitive-load)

## Intención

Transformar la interfaz de terminal (TUI) de SOLV Server Tracker en una herramienta de grado de sistemas quirúrgica y de baja carga cognitiva, basada en las lecciones heurísticas de k9s y la eliminación de los antipatrones de fatiga de paneles de Lazydocker/ctop. La TUI debe permitir a un operador evaluar la salud de su flota en menos de 1 segundo ("Regla del Primer Segundo"), predecir eventos de OOM al 85% de memoria Working Set, destacar sobrecostos de red mediante semáforos de Egress alineados a la derecha, navegar mediante atajos de teclado estilo Vim con filtrado reactivo (`/`) y ofrecer una transición limpia a pantalla completa para inspección de logs en vivo (`l` / `Enter`).

## Alcance

### Dentro del Alcance
- **Jerarquía Visual y Purga de Ruido**: Eliminación definitiva de PIDs, interfaces `veth` e IDs de 64 caracteres de la vista maestra; foco exclusivo en Señal: Estado, Nombre, CPU %, RAM Working Set y Egress.
- **Semántica de Glifos y Paleta Catppuccin Mocha**:
  - `[OK]` en verde (`#a6e3a1`) para estados normales.
  - `[--]` en gris atenuado (`#6c7086`) para contenedores detenidos intencionales.
  - `[||]` en amarillo (`#f9e2af`) para pausas.
  - `[!!]` en rojo vibrante (`#f38ba8`) para caídas y alertas críticas de OOM.
  - Cero emojis para garantizar consistencia entre emuladores de terminal.
- **Alerta Predictiva de OOM al 85%**: Detección visual temprana antes de que el kernel de Linux ejecute el OOM Killer.
- **Semáforo Topográfico de Egress**:
  - Valores alineados a la derecha (`lipgloss.Position(lipgloss.Right)`).
  - Escala cromática: `< 500 KB/s` (Subtext1), `500 KB/s - 5 MB/s` (Text), `5 MB/s - 50 MB/s` (Peach), `> 50 MB/s` (Red Bold).
- **Flujo Master-Detail a Pantalla Completa (Model Swapping)**: Transición sin paneles divididos pequeños hacia visor de logs en vivo (`viewport.Model`) con migas de pan (Breadcrumbs) en el Header (`[<] Volver (Esc) | Logs: <nombre> | Estado: [OK]`).
- **Filtrado Reactivo con `/`**: Búsqueda en tiempo real que colapsa la tabla dinámicamente mientras se tipea.
- **Blindaje ANSI y Consumo Zero-CPU**: Cálculo exacto de anchos con `lipgloss.Width()`, truncado pre-estilo y layouts colapsables en `View()`.

### Fuera del Alcance
- Paneles divididos fijos en 4 cuadrantes (descartado por inducir fatiga visual).
- Animaciones continuas o spinners a 30 FPS que consuman ciclos de CPU en reposo.

## Capacidades

### Nuevas Capacidades
- `tui-low-cognitive-load`: Interfaz de usuario de terminal optimizada para escaneo rápido en 1 segundo, atajos Vim, filtrado reactivo y visor de logs a pantalla completa.

### Capacidades Modificadas
- `agent-tui`: Actualización de la especificación anterior para incorporar la semántica de glifos ASCII, la alerta predictiva de OOM al 85% y el semáforo financiero de red.

## Enfoque Técnico

1. **Refactorización de `agent/internal/delivery/tui/tui.go`**:
   - Estructura `MainModel` con estados `stateFleetTable`, `stateLogViewer` y `stateFiltering`.
   - Implementación del componente `viewport.Model` para streaming de logs de Docker.
   - Integración de `textinput.Model` de Bubbles para el filtrado reactivo con `/`.
   - Cálculo de colores y estilos en función de umbrales numéricos de RAM (85%) y Egress.
