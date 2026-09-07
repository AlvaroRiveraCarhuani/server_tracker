# Especificación: hmac-ingestion-api

## Propósito
Validar criptográficamente cada petición de telemetría entrante antes de procesarla o persistirla en la base de datos.

## Requisitos y Escenarios

### Requisito: Validación de Firma HMAC-SHA256
La API DEBE verificar que la cabecera `X-Solv-Signature` coincida exactamente con `hmac_sha256(raw_body, shared_secret)`.

#### Escenario: Petición con firma válida
- **DADO** un payload de telemetría firmado con la clave compartida correcta
- **CUANDO** el agente realiza `POST /api/v1/telemetry/ingest`
- **ENTONCES** la API responde con código `200 OK` y persiste el lote.

#### Escenario: Rechazo de firma inválida o alterada
- **DADO** un payload con un cuerpo alterado o firmado con una clave incorrecta
- **CUANDO** se recibe la petición
- **ENTONCES** la API responde inmediatamente con código `401 Unauthorized` sin consultar la base de datos.

### Requisito: Mitigación de Ataques de Replay (Ventana de Timestamp)
La API DEBE rechazar cualquier petición cuyo timestamp en `X-Solv-Timestamp` difiera en más de 300 segundos respecto al reloj del servidor.

#### Escenario: Timestamp expirado
- **DADO** una petición capturada y reenviada con un timestamp de hace 10 minutos
- **CUANDO** la API evalúa la cabecera `X-Solv-Timestamp`
- **ENTONCES** DEBE responder con código `400 Bad Request` indicando timestamp expirado.
