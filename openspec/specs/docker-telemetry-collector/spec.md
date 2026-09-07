# Especificación: docker-telemetry-collector

## Propósito
Extraer y procesar métricas de CPU, memoria RAM y ancho de banda de red (Egress) directamente desde el socket de Docker (`/var/run/docker.sock`).

## Requisitos y Escenarios

### Requisito: Cálculo de Consumo de CPU
El recolector DEBE calcular el porcentaje de CPU utilizando deltas entre dos muestras temporales de la API de estadísticas de Docker.

#### Escenario: Muestreo de CPU con deltas válidos
- **DADO** un contenedor en ejecución con estadísticas previas y actuales
- **CUANDO** `cpu_delta > 0` y `system_cpu_delta > 0`
- **ENTONCES** el porcentaje de CPU se calcula como `(cpu_delta / system_cpu_delta) * num_cpus * 100.0`.

#### Escenario: Descarte de anomalías de CPU por reinicio
- **DADO** un contenedor que reinició su proceso o una lectura con `system_cpu_delta <= 0`
- **CUANDO** el recolector procesa las estadísticas
- **ENTONCES** DEBE descartar la muestra y devolver 0.0% sin generar valores negativos o NaN.

### Requisito: Cálculo de Consumo Real de RAM
El recolector DEBE reportar la memoria RAM neta utilizada por el contenedor, excluyendo el caché inactivo del sistema de archivos.

#### Escenario: Descuento de caché inactivo
- **DADO** un contenedor con `usage` total de memoria de 200 MB y `inactive_file` de 80 MB
- **CUANDO** se computa la métrica de memoria
- **ENTONCES** la métrica de RAM neta DEBE ser exactamente 120 MB.

### Requisito: Cálculo de Tasa de Egress de Red
El recolector DEBE calcular la tasa de transmisión de bytes (`tx_bytes`) por segundo a través de todas las interfaces de red del contenedor.

#### Escenario: Delta de bytes transmitidos
- **DADO** un intervalo de muestreo de 5 segundos con un incremento de 50 KB en `tx_bytes`
- **CUANDO** se calcula la tasa de salida
- **ENTONCES** la métrica de `egress_bytes_sec` DEBE ser de 10 KB/s.
