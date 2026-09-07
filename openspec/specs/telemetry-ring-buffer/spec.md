# Especificación: telemetry-ring-buffer

## Propósito
Proveer un buffer circular en memoria para acumular lotes de telemetría cuando el servidor backend esté inalcanzable, evitando fugas de memoria o bloqueo del agente.

## Requisitos y Escenarios

### Requisito: Límite de Capacidad en Memoria
El buffer circular DEBE mantener una cota fija de tamaño en bytes o número máximo de elementos (máximo 10 MB).

#### Escenario: Encolado normal de telemetría
- **DADO** un buffer con espacio disponible
- **CUANDO** el recolector genera un nuevo lote de métricas
- **ENTONCES** el elemento se almacena al final de la cola y el tamaño aumenta.

#### Escenario: Descarte FIFO ante desbordamiento
- **DADO** un buffer lleno a su máxima capacidad por desconexión prolongada
- **CUANDO** se intenta insertar un nuevo lote de telemetría
- **ENTONCES** DEBE descartar el lote más antiguo (FIFO) e insertar el nuevo sin superar el límite de memoria ni arrojar pánico.

#### Escenario: Drenado secuencial tras reconexión
- **DADO** un buffer con elementos acumulados y conexión restablecida con el servidor
- **CUANDO** el cliente de transporte solicita lotes pendientes
- **ENTONCES** DEBE vaciar y enviar los lotes en orden cronológico.
