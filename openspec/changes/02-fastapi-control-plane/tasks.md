# Tareas: Control Plane en FastAPI (02-fastapi-control-plane)

## Fase 1: Autenticación HMAC y Schemas (TDD)
- [x] 1.1 Escribir tests en `tests/test_hmac_auth.py` para firma válida, firma alterada y timestamp expirado.
- [x] 1.2 Implementar esquemas Pydantic v2 en `schemas/telemetry.py`.
- [x] 1.3 Implementar dependencia de verificación HMAC en `auth/hmac_auth.py`.
- [x] 1.4 Verificar que los tests de autenticación pasen.

## Fase 2: Modelos de Base de Datos y Persistencia
- [x] 2.1 Definir modelos SQLAlchemy en `models/metrics.py` para `Host` y `ContainerMetric`.
- [x] 2.2 Configurar sesión y motor en `database.py`.
- [x] 2.3 Escribir tests de integración de base de datos en `tests/test_telemetry_ingest.py`.

## Fase 3: Router de Telemetría e Ingestión
- [x] 3.1 Implementar endpoint `POST /api/v1/telemetry/ingest` con validación HMAC y guardado atómico.
- [x] 3.2 Implementar endpoints de lectura `GET /api/v1/telemetry/hosts` y `GET /api/v1/telemetry/{host_id}/live`.
- [x] 3.3 Montar router en `main.py` y validar con tests de pytest.

## Fase 4: Servidor MCP para IA (AIOps)
- [x] 4.1 Escribir tests para herramientas MCP en `tests/test_mcp_tools.py`.
- [x] 4.2 Implementar herramientas MCP en `mcp_server.py` (`get_infrastructure_overview`, `detect_anomalies_and_egress_spikes`).

## Fase 5: Telegram ChatOps Seguro (D5)
- [x] 5.1 Escribir tests para generación y validación de tokens efímeros en `tests/test_telegram_guard.py`.
- [x] 5.2 Implementar botones con HMAC y TTL de 60s en `notifications/telegram.py`.
- [x] 5.3 Implementar receptor de webhook en `routers/telegram_webhook.py`.
- [x] 5.4 Ejecutar todo el conjunto de pruebas de pytest y asegurar 100% verde.
