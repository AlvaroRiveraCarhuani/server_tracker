# Especificación: remediation-audit-log

## Propósito
Registrar de forma inmutable y auditable toda acción de remediación ejecutada sobre los contenedores de la infraestructura.

## Requisitos y Escenarios

### Requisito: Registro Inmutable de Auditoría
Toda orden de control ejecutada vía Telegram o MCP DEBE registrarse en la tabla `audit_logs` con su resultado y operador responsable.

#### Escenario: Auditoría de reinicio exitoso
- **DADO** un operador que presiona el botón de reinicio en Telegram
- **CUANDO** la acción finaliza
- **ENTONCES** se inserta un registro con `host_id`, `container_id`, `action="restart"`, `source="telegram"`, `operator_id` y `status="success"`.
