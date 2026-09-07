# Especificación: keyring-fallback-vault

## Propósito
Gestionar el almacenamiento persistente y seguro de credenciales (`server_url`, `agent_secret`) sin archivos de texto plano `.env`.

## Requisitos y Escenarios

### Requisito: Cadena de Fallback en Cascada
El gestor de credenciales DEBE intentar acceder a los almacenes en el orden estricto: Keyring nativo $\rightarrow$ Bóveda cifrada local $\rightarrow$ Variables de entorno.

#### Escenario: Recuperación exitosa desde Keyring del SO
- **DADO** un entorno de escritorio o servidor con D-Bus / SecretService disponible
- **CUANDO** el agente inicializa la carga de credenciales
- **ENTONCES** DEBE recuperar el secreto directamente del Keyring del sistema.

#### Escenario: Fallback a bóveda cifrada en servidor Headless
- **DADO** un servidor SSH sin sesión D-Bus activa
- **CUANDO** el Keyring del SO arroja error de indisponibilidad
- **ENTONCES** DEBE leer el archivo `~/.solv/vault.enc` descifrándolo mediante AES-256-GCM con la clave derivada vía Argon2id.

#### Escenario: Permisos estrictos de la bóveda local
- **DADO** que se crea la bóveda local `~/.solv/vault.enc`
- **CUANDO** se escribe el archivo en el sistema de archivos
- **ENTONCES** los permisos del archivo DEBEN ser exactamente `0600` (lectura/escritura exclusiva del usuario).
