# Propuesta: Diagnóstico Pasivo Zero-Prompt con OpenRouter en la TUI (06-tui-zero-prompt-ai)

## Intención

Integrar inteligencia analítica desatendida directamente en la TUI de Go. Cuando un contenedor presente anomalías (`STOPPED`, `OOM_RISK`, `restarting`, `dead`), el agente extraerá los últimos logs de error y consultará en segundo plano a OpenRouter, desplegando un banner contextual con la causa raíz en 1 línea sin que el operador tenga que escribir ningún prompt.

## Alcance

### Dentro del Alcance
- **Caché en Memoria de Diagnósticos**: Mapa `containerID -> diagnosis` para evitar consultas redundantes y no agotar la cuota de OpenRouter.
- **Consulta Asíncrona sin Bloqueo (`tea.Cmd`)**: Cliente HTTP liviano en Go hacia `https://openrouter.ai/api/v1/chat/completions` con timeout de 6 segundos.
- **Banner Contextual AIOps en la TUI**: Panel visual sobrio en Catppuccin Mocha ubicado entre la tabla de contenedores y la barra de estado cuando el contenedor seleccionado presente anomalías.
- **Estado de Carga y Fallback Elegante**: Si la API está saturada (HTTP 429) o no hay conectividad, muestra sugerencia local basada en el código de salida y estado.
- **Tests Automatizados**: Pruebas unitarias para el despachador de diagnóstico y renderizado del banner.

### Fuera del Alcance
- Modales interactivos de chat (el flujo es estrictamente Zero-Prompt).
- Consultas periódicas a contenedores en estado `RUNNING` saludable.

## Capacidades

### Nuevas Capacidades
- `tui-zero-prompt-ai`: Detección reactiva de anomalías y renderizado de diagnóstico sintetizado en 1 línea sin fricción.
