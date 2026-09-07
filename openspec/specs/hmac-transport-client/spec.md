# Especificación: hmac-transport-client

## Propósito
Transmitir la telemetría recolectada mediante un canal saliente reverso con firma criptográfica HMAC-SHA256 y recibir órdenes de remediación autorizadas.

## Requisitos y Escenarios

### Requisito: Firma Criptográfica de Payloads
Cada paquete HTTP/WebSocket saliente DEBE incluir la cabecera `X-Solv-Signature` generada con HMAC-SHA256 usando el secreto del agente.

#### Escenario: Envío de telemetría firmada
- **DADO** un payload JSON de métricas y un secreto de agente válido
- **CUANDO** el cliente envía la petición al servidor
- **ENTONCES** DEBE calcular `hmac_sha256(payload, secret)` e incluir el hash en la cabecera `X-Solv-Signature` junto con el timestamp UTC.

### Requisito: Canal Reverso y Restricción de Comandos (Cero RCE)
El cliente DEBE procesar únicamente órdenes de remediación predefinidas provenientes del canal reverso.

#### Escenario: Ejecución de acción permitida
- **DADO** un mensaje entrante firmado solicitando la acción `restart` sobre un contenedor
- **CUANDO** el agente valida la firma y el comando
- **ENTONCES** DEBE invocar la API de Docker para reiniciar el contenedor especificado.

#### Escenario: Rechazo de acciones arbitrarias no autorizadas
- **DADO** un mensaje solicitando `exec` o ejecución de comandos de shell
- **CUANDO** el agente inspecciona la orden
- **ENTONCES** DEBE rechazar la petición inmediatamente y registrar una advertencia de seguridad.
