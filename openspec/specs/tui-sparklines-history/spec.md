# Especificación: Sparklines y Tendencias en la TUI

## Requisitos de Capacidad y Memoria
- El buffer de histórico no debe almacenar más de 30 muestras temporales por contenedor para mantener el consumo de memoria del agente en Go en menos de unos pocos kilobytes.
- Cuando un contenedor deja de ser reportado por Docker, sus entradas en el mapa de histórico deben ser eliminadas en un ciclo de limpieza para evitar fugas.

## Requisitos Visuales
- La sparkline debe emplear caracteres en bloque Unicode estandarizados: ` ' ', '▂', '▃', '▄', '▅', '▆', '▇', '█' `.
- Si los datos son insuficientes (p. ej. menos de 2 muestras), debe renderizar puntos o guiones sutiles sin provocar pánico o división por cero.
- La semántica cromática debe mantenerse consistente con el diseño general (Catppuccin Mocha): verde para rangos estables, durazno para advertencia y rojo para picos críticos.
