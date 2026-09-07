# Especificación: timescale-metrics-storage

## Propósito
Modelar y persistir series temporales de métricas de contenedores con soporte para consultas de alta velocidad y retención eficiente.

## Requisitos y Escenarios

### Requisito: Registro Atómico de Lote de Métricas
El sistema DEBE registrar todas las métricas de un lote de telemetría de forma atómica en una única transacción de base de datos.

#### Escenario: Inserción de lote con múltiples contenedores
- **DADO** un lote con 5 contenedores de un host
- **CUANDO** se procesa la ingestión
- **ENTONCES** se insertan 5 registros en `container_metrics` y se actualiza `last_seen_at` en la tabla `hosts`.

### Requisito: Consulta de Métricas Recientes
La API DEBE proveer un endpoint para obtener el último estado conocido de los contenedores por host.

#### Escenario: Consulta de estado en vivo
- **DADO** un host con métricas registradas en los últimos 60 segundos
- **CUANDO** un cliente consulta `GET /api/v1/telemetry/{host_id}/live`
- **ENTONCES** la API devuelve la lista de contenedores con sus valores de CPU %, RAM, Egress y estado actual.
