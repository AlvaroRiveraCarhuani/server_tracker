# Propuesta: Control Plane en FastAPI (Ingestión HMAC, TimescaleDB, Servidor MCP y ChatOps)

## Intención

Implementar el Control Plane (servidor central) en Python con FastAPI para SOLV Server Tracker. El servidor se encarga de recibir e ingerir la telemetría enviada por los agentes Go previa validación de firma HMAC-SHA256, almacenar series temporales en PostgreSQL/TimescaleDB, exponer herramientas de infraestructura para modelos de lenguaje mediante el Model Context Protocol (MCP), y gestionar alertas y remediación interactiva mediante Telegram ChatOps blindado con tokens efímeros.

## Alcance

### Dentro del Alcance
- **Middleware de Autenticación Zero-Trust HMAC**: Validación de `X-Solv-Signature` y `X-Solv-Timestamp` (ventana máxima de 300s para mitigar ataques de replay).
- **Esquema de Base de Datos y Almacenamiento**: Tablas `hosts` y `container_metrics` con soporte para series de tiempo y SQLAlchemy asíncrono/síncrono.
- **API de Telemetría**: Endpoints de ingestión (`POST /api/v1/telemetry/ingest`) y consulta de estado (`GET /api/v1/telemetry/hosts`, `GET /api/v1/telemetry/{host_id}/live`).
- **Servidor MCP (Model Context Protocol)**: Herramientas nativas para orquestadores de IA (obtención de métricas, diagnóstico de anomalías de RAM/OOMKilled y detección de picos de egress de red).
- **Telegram ChatOps Blindado (D5)**: Envío de alertas interactivas con botones que portan un token firmado con HMAC y TTL de 60 segundos, más verificación de lista blanca de `chat_id` / `user_id`.
- **Conjunto de Tests Automatizados con Pytest**: Tests para validación HMAC, ingestión de telemetría, rechazo de firmas alteradas y flujo de ChatOps.

### Fuera del Alcance
- Interfaz gráfica web compleja en React/Vue (la interacción se realiza vía TUI, Telegram ChatOps y Servidor MCP).
- Ejecución de comandos arbitrarios sin lista blanca (`exec` prohibido).

## Capacidades

### Nuevas Capacidades
- `hmac-ingestion-api`: Endpoint de ingestión de telemetría con verificación de firma HMAC-SHA256 y control de replay.
- `timescale-metrics-storage`: Modelo de persistencia para series temporales de métricas de contenedores.
- `mcp-infrastructure-server`: Exposición de herramientas de monitoreo y diagnóstico para LLMs mediante el protocolo MCP.
- `telegram-chatops-guard`: Notificaciones interactivas y recepción de callbacks con tokens efímeros firmados (TTL 60s).

## Enfoque Técnico

1. **Estructura Modular**:
   - `auth/hmac_auth.py`: Validador de firma HMAC y timestamps.
   - `models/metrics.py`: Modelos SQLAlchemy para hosts y métricas.
   - `schemas/telemetry.py`: Schemas de validación Pydantic v2.
   - `routers/telemetry.py`: Endpoints REST de ingestión y consulta.
   - `mcp_server.py`: Definición de herramientas MCP para IA.
   - `notifications/telegram.py` y `routers/telegram_webhook.py`: ChatOps interactivo.
2. **Estrategia de Pruebas (Strict TDD)**:
   - `tests/test_hmac_auth.py`: Pruebas de firmas válidas, inválidas, expiradas y payloads manipulados.
   - `tests/test_telemetry_ingest.py`: Pruebas de ingestión y persistencia con base de datos de test SQLite/Postgres.
   - `tests/test_mcp_tools.py`: Pruebas de ejecución de herramientas MCP.
