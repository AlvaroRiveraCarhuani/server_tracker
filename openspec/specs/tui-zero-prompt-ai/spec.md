# Especificación: Diagnóstico Pasivo Zero-Prompt con OpenRouter en la TUI (tui-zero-prompt-ai)

## Requisitos

### R1: Detección Automática de Anomalías
- Se considera contenedor anómalo todo contenedor que:
  - Tenga `Status != "running"` (ej. `stopped`, `exited`, `dead`, `restarting`).
  - O tenga ratio de memoria `ramBytes / ramLimit >= 0.85` (`OOM_RISK`).

### R2: Caché Local de Diagnóstico
- Cada contenedor anómalo diagnosticado se indexa en memoria por su ID (`map[string]string`).
- Si el ID ya existe en caché, la TUI muestra el resultado inmediatamente sin realizar llamadas de red.

### R3: Inferencia Desatendida Asíncrona
- La solicitud a OpenRouter se ejecuta en una goroutine aislada mediante `tea.Cmd`.
- La TUI nunca se bloquea ni ralentiza el framerate durante la petición HTTP.
- Si no hay API key de OpenRouter configurada o la API está saturada, muestra un mensaje descriptivo sin error bloqueante.

### R4: Banner Contextual Discreto
- El banner se renderiza debajo de la tabla únicamente cuando el cursor apunta a un contenedor anómalo.
- Formato: `[AIOps] <Diagnóstico en 1 línea>` con paleta Catppuccin Mocha.
