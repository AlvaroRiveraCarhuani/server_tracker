# Especificación: tui-low-cognitive-load

## Propósito
Definir los contratos de diseño, interacción y rendimiento de la interfaz de terminal (TUI) para garantizar la mínima carga cognitiva y la máxima velocidad de respuesta ante incidentes.

## Requisitos y Escenarios

### Requisito: Semántica de Glifos y Colores Catppuccin Mocha (Cero Emojis)
La TUI DEBE utilizar combinaciones de corchetes ASCII con colores de Catppuccin Mocha para representar la salud del contenedor sin provocar desfasajes de columnas.

#### Escenario: Renderizado de contenedor saludable
- **DADO** un contenedor en ejecución normal
- **CUANDO** se renderiza su fila en la tabla
- **ENTONCES** la columna de estado muestra `[OK]` en verde (`#a6e3a1`).

#### Escenario: Renderizado de contenedor detenido
- **DADO** un contenedor en estado `exited` o `stopped`
- **CUANDO** se renderiza en la tabla
- **ENTONCES** muestra `[--]` en gris apagado (`#6c7086`) para hundirse en el fondo visual.

#### Escenario: Detección predictiva de OOM al 85%
- **DADO** un contenedor cuyo Working Set de RAM supera el 85% del límite asignado
- **CUANDO** la TUI computa el estado
- **ENTONCES** DEBE mostrar `[!!]` en rojo vibrante (`#f38ba8`) con texto `OOM_RISK` para alertar antes de la acción del kernel.

### Requisito: Semáforo de Egress Financiero Alineado a la Derecha
La columna de tráfico de salida DEBE estar estrictamente alineada a la derecha y aplicar semáforos cromáticos según el volumen de datos.

#### Escenario: Egress superior al umbral de sobrecosto (>50 MB/s)
- **DADO** un contenedor con tasa de salida de 65 MB/s
- **CUANDO** se pinta la columna de Egress
- **ENTONCES** el texto se formatea alineado a la derecha en rojo (`#f38ba8`) en negrita.

### Requisito: Transición Master-Detail a Pantalla Completa (Logs en Vivo)
Al presionar `l` o `Enter` sobre un contenedor, la tabla de la flota DEBE ser reemplazada completamente por el visor de logs a pantalla completa.

#### Escenario: Inspección de logs y regreso con Esc
- **DADO** el usuario enfocado en un contenedor específico
- **CUANDO** presiona `l` o `Enter`
- **ENTONCES** la interfaz conmuta al visor de logs al 100% de ancho con breadcrumb `[<] Volver (Esc) | Logs: <nombre>`, y al presionar `Esc` regresa a la flota sin pérdida de posición de cursor.

### Requisito: Filtrado Reactivo con Tecla `/`
La TUI DEBE permitir filtrar contenedores en tiempo real al pulsar la tecla `/`.

#### Escenario: Búsqueda reactiva
- **DADO** una lista de 20 contenedores
- **CUANDO** el usuario presiona `/` y escribe `redis`
- **ENTONCES** la tabla colapsa instantáneamente mostrando solo los contenedores cuyo nombre contenga `redis`.
