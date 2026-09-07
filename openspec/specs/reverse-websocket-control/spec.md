# Especificación: reverse-websocket-control

## Propósito
Proveer un canal de comunicación saliente seguro y persistente desde el agente Go hacia FastAPI para la ejecución en caliente de órdenes de remediación autorizadas.

## Requisitos y Escenarios

### Requisito: Autenticación HMAC en el Handshake WebSocket
El cliente Go DEBE autenticarse durante el handshake enviando `X-Solv-Signature` y `X-Solv-Timestamp` calculados sobre el `host_id` y timestamp.

#### Escenario: Conexión exitosa
- **DADO** un agente con clave válida
- **CUANDO** inicia la conexión a `WS /api/v1/ws/agent/{host_id}`
- **ENTONCES** el servidor valida la firma y mantiene la conexión viva, registrando el host como activo.

#### Escenario: Rechazo de conexión no autorizada
- **DADO** una firma inválida o timestamp expirado
- **CUANDO** se intenta abrir el WebSocket
- **ENTONCES** el servidor cierra la conexión con código de error `4401 Unauthorized`.

### Requisito: Despacho de Órdenes y Confirmación (ACK)
El servidor DEBE poder despachar comandos JSON y recibir confirmaciones del agente.

#### Escenario: Ejecución de reinicio
- **DADO** un agente conectado por WebSocket
- **CUANDO** el servidor envía `{"id": "cmd-1", "action": "restart", "container_id": "c1"}`
- **ENTONCES** el agente ejecuta la acción en Docker y responde con `{"id": "cmd-1", "success": true, "error": null}`.
