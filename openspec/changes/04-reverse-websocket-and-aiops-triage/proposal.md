# Propuesta: Canal Reverso WebSocket y Triaje AIOps con OpenRouter (04-reverse-websocket-and-aiops-triage)

## Intención

Implementar el canal de control reverso saliente (WebSocket) entre el Agente en Go y el Control Plane en FastAPI, permitiendo el despacho y la ejecución en caliente de órdenes de remediación autorizadas (`restart`, `stop`, `isolate_network`) sin abrir puertos en el host anfitrión. Además, integrar el servicio de triaje AIOps utilizando modelos de inferencia a través de OpenRouter para analizar fallos de contenedores en tiempo real y enviar diagnósticos inteligentes con botones interactivos a Telegram ChatOps.

## Alcance

### Dentro del Alcance
- **Canal Reverso WebSocket Seguro**:
  - Endpoint en FastAPI: `WS /api/v1/ws/agent/{host_id}` con autenticación criptográfica HMAC-SHA256 en el handshake.
  - Conexión cliente saliente en Go con reconexión automática (Exponential Backoff y Jitter) y Ping/Pong cada 15 segundos.
  - Protocolo de mensajería tipado para órdenes de control y confirmaciones (ACKs).
- **Despacho Bidireccional de Remediación**:
  - Recepción de callback de Telegram o petición MCP $\rightarrow$ Despacho por WebSocket al agente $\rightarrow$ Ejecución en Docker $\rightarrow$ Respuesta con resultado.
  - Whitelist estricta de acciones: `restart`, `stop`, `isolate_network` (Cero RCE).
- **Servicio de Triaje AIOps con OpenRouter**:
  - Módulo `services/ai_triage.py` que consulta la API de OpenRouter (`meta-llama/llama-3.3-70b-instruct:free`, `google/gemini-2.0-flash-exp:free`) enviando los últimos logs y estado del contenedor caído.
  - Síntesis en 2 líneas del problema para acompañar la alerta interactiva en Telegram.
- **Tabla de Auditoría en PostgreSQL**:
  - Modelo `AuditLog` para registrar cada orden ejecutada (timestamp, host_id, container_id, acción, origen, resultado).
- **Tests Automatizados**:
  - Tests unitarios y de integración para el handshake WebSocket, despacho de órdenes y servicio de triaje con mock.

### Fuera del Alcance
- Ejecución de comandos de shell arbitrarios (`docker exec` prohibido por D1).
- Dependencia de un único proveedor de IA propietario (agnóstico vía OpenRouter / OpenAI spec).

## Capacidades

### Nuevas Capacidades
- `reverse-websocket-control`: Canal de comunicación bidireccional saliente y despacho seguro de acciones.
- `aiops-telegram-triage`: Diagnóstico automatizado de incidentes con LLM vía OpenRouter.
- `remediation-audit-log`: Trazabilidad inmutable de todas las acciones de infraestructura ejecutadas.
