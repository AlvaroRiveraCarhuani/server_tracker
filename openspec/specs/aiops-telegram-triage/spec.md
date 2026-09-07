# Especificación: aiops-telegram-triage

## Propósito
Analizar registros y estados de fallo de contenedores utilizando modelos de inferencia a través de OpenRouter para entregar diagnósticos comprensibles en Telegram.

## Requisitos y Escenarios

### Requisito: Síntesis de Fallo en Lenguaje Natural
El servicio DEBE procesar las últimas líneas de registro de un contenedor caído y generar un diagnóstico técnico de 2 a 3 líneas con sugerencia de remediación.

#### Escenario: Análisis de contenedor con OOM
- **DADO** un log que contiene `fatal error: runtime: out of memory`
- **CUANDO** el triaje analiza el evento
- **ENTONCES** genera una explicación concisa del agotamiento de memoria y recomienda reiniciar y ajustar límites.
