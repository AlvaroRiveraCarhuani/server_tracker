package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/ports"
	"github.com/zalando/go-keyring"
	"golang.org/x/crypto/argon2"
)

const (
	ServiceName = "solv_server_tracker"
	saltLen     = 16
	nonceLen    = 12
	keyLen      = 32
)

var (
	ErrCredentialsNotFound = errors.New("credenciales no encontradas en el almacén")
)

type credentialsPayload struct {
	ServerURL   string `json:"server_url"`
	SecretToken string `json:"secret_token"`
}

// FileVault implementa ports.VaultPort guardando credenciales cifradas con AES-256-GCM + Argon2id.
type FileVault struct {
	path       string
	passphrase string
}

// NewFileVault crea una instancia de bóveda basada en archivo local cifrado.
func NewFileVault(path, passphrase string) *FileVault {
	return &FileVault{
		path:       path,
		passphrase: passphrase,
	}
}

func (v *FileVault) Save(serverURL, secretToken string) error {
	dir := filepath.Dir(v.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("error creando directorio para bóveda: %w", err)
	}

	payload, err := json.Marshal(credentialsPayload{
		ServerURL:   serverURL,
		SecretToken: secretToken,
	})
	if err != nil {
		return fmt.Errorf("error serializando credenciales: %w", err)
	}

	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return fmt.Errorf("error generando salt: %w", err)
	}

	// Derivación de clave con Argon2id: time=1, memory=64MB, threads=4, keyLen=32
	key := argon2.IDKey([]byte(v.passphrase), salt, 1, 64*1024, 4, keyLen)

	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("error creando cifrador AES: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("error inicializando GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("error generando nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, payload, nil)

	// Estructura en disco: [SALT (16B)] + [NONCE (12B)] + [CIPHERTEXT]
	finalData := append(salt, nonce...)
	finalData = append(finalData, ciphertext...)

	// Escribir con permisos estrictos 0600 (D2)
	return os.WriteFile(v.path, finalData, 0600)
}

func (v *FileVault) Get() (string, string, error) {
	data, err := os.ReadFile(v.path)
	if err != nil {
		return "", "", fmt.Errorf("%w: %v", ErrCredentialsNotFound, err)
	}

	minLen := saltLen + nonceLen
	if len(data) <= minLen {
		return "", "", errors.New("archivo de bóveda corrupto o incompleto")
	}

	salt := data[:saltLen]
	nonce := data[saltLen:minLen]
	ciphertext := data[minLen:]

	key := argon2.IDKey([]byte(v.passphrase), salt, 1, 64*1024, 4, keyLen)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", "", fmt.Errorf("error creando cifrador AES: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", fmt.Errorf("error inicializando GCM: %w", err)
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", "", fmt.Errorf("fallo de autenticación/descifrado (contraseña incorrecta o datos alterados): %w", err)
	}

	var creds credentialsPayload
	if err := json.Unmarshal(plaintext, &creds); err != nil {
		return "", "", fmt.Errorf("error deserializando credenciales: %w", err)
	}

	return creds.ServerURL, creds.SecretToken, nil
}

// KeyringVault implementa ports.VaultPort usando el Keyring nativo del SO.
type KeyringVault struct{}

func NewKeyringVault() *KeyringVault {
	return &KeyringVault{}
}

func (k *KeyringVault) Save(serverURL, secretToken string) error {
	if err := keyring.Set(ServiceName, "server_url", serverURL); err != nil {
		return err
	}
	return keyring.Set(ServiceName, "secret_token", secretToken)
}

func (k *KeyringVault) Get() (string, string, error) {
	url, err := keyring.Get(ServiceName, "server_url")
	if err != nil {
		return "", "", err
	}
	token, err := keyring.Get(ServiceName, "secret_token")
	if err != nil {
		return "", "", err
	}
	return url, token, nil
}

// EnvVault implementa ports.VaultPort leyendo variables de entorno para CI/CD.
type EnvVault struct{}

func NewEnvVault() *EnvVault {
	return &EnvVault{}
}

func (e *EnvVault) Save(serverURL, secretToken string) error {
	return errors.New("no se admite escritura en variables de entorno")
}

func (e *EnvVault) Get() (string, string, error) {
	url := os.Getenv("SOLV_SERVER_URL")
	token := os.Getenv("SOLV_AGENT_SECRET")
	if url == "" || token == "" {
		return "", "", ErrCredentialsNotFound
	}
	return url, token, nil
}

// CascadeVault implementa ports.VaultPort intentando en orden: Keyring -> Archivo cifrado -> Env vars.
type CascadeVault struct {
	keyring   *KeyringVault
	fileVault *FileVault
	envVault  *EnvVault
}

func NewCascadeVault(fileVaultPath, passphrase string) ports.VaultPort {
	return &CascadeVault{
		keyring:   NewKeyringVault(),
		fileVault: NewFileVault(fileVaultPath, passphrase),
		envVault:  NewEnvVault(),
	}
}

func (c *CascadeVault) Save(serverURL, secretToken string) error {
	// Intentamos primero Keyring del SO
	err := c.keyring.Save(serverURL, secretToken)
	if err == nil {
		return nil
	}
	// Si falla (headless/SSH), usamos la bóveda de archivo local
	return c.fileVault.Save(serverURL, secretToken)
}

func (c *CascadeVault) Get() (string, string, error) {
	// 1. Intentar Keyring del SO
	url, token, err := c.keyring.Get()
	if err == nil && url != "" && token != "" {
		return url, token, nil
	}

	// 2. Intentar Bóveda de Archivo Cifrado
	url, token, err = c.fileVault.Get()
	if err == nil && url != "" && token != "" {
		return url, token, nil
	}

	// 3. Intentar Variables de Entorno (CI/CD)
	url, token, err = c.envVault.Get()
	if err == nil && url != "" && token != "" {
		return url, token, nil
	}

	return "", "", ErrCredentialsNotFound
}
