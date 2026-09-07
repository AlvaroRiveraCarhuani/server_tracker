# Especificación: mcp-infrastructure-server

## Propósito
Exponer el estado de los servidores, contenedores y anomalías operativas como herramientas nativas MCP para modelos de lenguaje (Claude Code, Cursor, OpenCode).

## Requisitos y Escenarios

### Requisito: Herramienta de Resumen de Infraestructura
El servidor MCP DEBE proveer la herramienta `get_infrastructure_overview` listando todos los hosts conectados, contenedores activos y alertas globales.

#### Escenario: Consulta de estado general por IA
- **DADO** un modelo de IA conectado vía MCP
- **CUANDO** invoca `get_infrastructure_overview()`
- **ENTONCES** recibe un resumen estructurado con total de hosts, contenedores arriba/abajo y consumo consolidado.

### Requisito: Detección de Anomalías y Picos de Egress
El servidor MCP DEBE proveer la herramienta `detect_anomalies_and_egress_spikes` para identificar contenedores que exceden límites de RAM o ancho de banda.

#### Escenario: Detección de contenedor con consumo anómalo
- **DADO** un contenedor con tasa de salida superior a 50 MB/s o 95% de RAM
- **CUANDO** la IA ejecuta la herramienta de anomalías
- **ENTONCES** devuelve el ID del contenedor, severidad de la anomalía y recomendación de remediación.
