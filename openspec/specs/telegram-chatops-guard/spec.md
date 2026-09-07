# Especificación: telegram-chatops-guard

## Propósito
Blindar las operaciones de remediación por Telegram ChatOps mediante tokens efímeros firmados con HMAC y lista blanca de usuarios autorizados.

## Requisitos y Escenarios

### Requisito: Generación de Callback Seguro con TTL
Cada botón interactivo enviado a Telegram DEBE codificar una acción, un host, un contenedor, un timestamp y una firma HMAC con validez máxima de 60 segundos.

#### Escenario: Envío de alerta con botón firmado
- **DADO** un contenedor que se detuvo inesperadamente
- **CUANDO** el bot envía la notificación a Telegram
- **ENTONCES** el `callback_data` contiene el formato `act:{action}:{host_id}:{container_id}:{timestamp}:{signature_preview}`.

### Requisito: Validación Estricta de Webhook de Telegram
Al recibir un callback de Telegram, el webhook DEBE validar la lista blanca de `from.id` y la vigencia del token HMAC.

#### Escenario: Ejecución de callback legítimo dentro del TTL
- **DADO** un operador autorizado que presiona el botón antes de 60 segundos
- **CUANDO** el webhook procesa la acción
- **ENTONCES** valida la firma, ejecuta la orden en el canal reverso y actualiza el mensaje en Telegram confirmando el reinicio.

#### Escenario: Rechazo de callback expirado o reenviado
- **DADO** un botón presionado después de 60 segundos o por un usuario no autorizado
- **CUANDO** se recibe el webhook
- **ENTONCES** DEBE rechazar la acción respondiendo con una alerta de expiración o falta de permisos.
