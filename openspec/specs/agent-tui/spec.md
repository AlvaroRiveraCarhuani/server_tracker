# Especificación: agent-tui

## Propósito
Proveer una interfaz de terminal interactiva (TUI) ligera construida con Bubbletea para visualizar métricas en tiempo real sin salir del flujo de trabajo de la terminal.

## Requisitos y Escenarios

### Requisito: Límite de Frecuencia de Actualización (Framerate)
La TUI DEBE limitar su bucle de renderizado a un máximo de 2 fotogramas por segundo (2 FPS) para evitar sobrecargar la CPU del servidor.

#### Escenario: Renderizado periódico
- **DADO** la TUI en primer plano mostrando métricas
- **CUANDO** transcurre un segundo
- **ENTONCES** no se deben emitir más de 2 redibujados completos de pantalla salvo interacción del teclado.

### Requisito: Atajos de Teclado de una Sola Tecla (Single-Key)
La navegación y las acciones comunes DEBEN ser accesibles con una sola tecla.

#### Escenario: Navegación de contenedores
- **DADO** la lista de contenedores en pantalla
- **CUANDO** el usuario presiona `j` o `k` (o flechas arriba/abajo)
- **ENTONCES** la selección se desplaza al contenedor correspondiente.

#### Escenario: Salida limpia
- **DADO** la TUI en ejecución
- **CUANDO** el usuario presiona `q` o `Ctrl+C`
- **ENTONCES** la aplicación restaura la terminal y finaliza sin dejar artefactos de escape ANSI rotos.
