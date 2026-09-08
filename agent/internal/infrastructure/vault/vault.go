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

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
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
	ServerURL     string           `json:"server_url"`
	SecretToken   string           `json:"secret_token"`
	OpenRouterKey string           `json:"openrouter_key,omitempty"`
	AIConfig      *domain.AIConfig `json:"ai_config,omitempty"`
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

func (v *FileVault) writePayload(creds credentialsPayload) error {
	dir := filepath.Dir(v.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("error creando directorio para bóveda: %w", err)
	}

	payload, err := json.Marshal(creds)
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

func (v *FileVault) readPayload() (credentialsPayload, error) {
	var creds credentialsPayload
	data, err := os.ReadFile(v.path)
	if err != nil {
		return creds, fmt.Errorf("%w: %v", ErrCredentialsNotFound, err)
	}

	minLen := saltLen + nonceLen
	if len(data) <= minLen {
		return creds, errors.New("archivo de bóveda corrupto o incompleto")
	}

	salt := data[:saltLen]
	nonce := data[saltLen:minLen]
	ciphertext := data[minLen:]

	key := argon2.IDKey([]byte(v.passphrase), salt, 1, 64*1024, 4, keyLen)

	block, err := aes.NewCipher(key)
	if err != nil {
		return creds, fmt.Errorf("error creando cifrador AES: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return creds, fmt.Errorf("error inicializando GCM: %w", err)
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return creds, fmt.Errorf("fallo de autenticación/descifrado (contraseña incorrecta o datos alterados): %w", err)
	}

	if err := json.Unmarshal(plaintext, &creds); err != nil {
		return creds, fmt.Errorf("error deserializando credenciales: %w", err)
	}

	return creds, nil
}

func (v *FileVault) Save(serverURL, secretToken string) error {
	creds, _ := v.readPayload()
	creds.ServerURL = serverURL
	creds.SecretToken = secretToken
	return v.writePayload(creds)
}

func (v *FileVault) Get() (string, string, error) {
	creds, err := v.readPayload()
	if err != nil {
		return "", "", err
	}
	return creds.ServerURL, creds.SecretToken, nil
}

func (v *FileVault) SaveOpenRouterKey(key string) error {
	creds, _ := v.readPayload()
	creds.OpenRouterKey = key
	if creds.AIConfig == nil {
		cfg := domain.DefaultAIConfig()
		creds.AIConfig = &cfg
	}
	p := creds.AIConfig.Providers[domain.ProviderOpenRouter]
	p.APIKey = key
	creds.AIConfig.Providers[domain.ProviderOpenRouter] = p
	return v.writePayload(creds)
}

func (v *FileVault) GetOpenRouterKey() (string, error) {
	creds, err := v.readPayload()
	if err != nil {
		return "", err
	}
	if creds.AIConfig != nil {
		if p, ok := creds.AIConfig.Providers[domain.ProviderOpenRouter]; ok && p.APIKey != "" {
			return p.APIKey, nil
		}
	}
	if creds.OpenRouterKey == "" {
		return "", ErrCredentialsNotFound
	}
	return creds.OpenRouterKey, nil
}

func (v *FileVault) SaveAIConfig(cfg domain.AIConfig) error {
	creds, _ := v.readPayload()
	creds.AIConfig = &cfg
	if p, ok := cfg.Providers[domain.ProviderOpenRouter]; ok && p.APIKey != "" {
		creds.OpenRouterKey = p.APIKey
	}
	return v.writePayload(creds)
}

func (v *FileVault) GetAIConfig() (domain.AIConfig, error) {
	creds, err := v.readPayload()
	if err != nil {
		return domain.DefaultAIConfig(), err
	}
	if creds.AIConfig != nil {
		return *creds.AIConfig, nil
	}
	cfg := domain.DefaultAIConfig()
	if creds.OpenRouterKey != "" {
		p := cfg.Providers[domain.ProviderOpenRouter]
		p.APIKey = creds.OpenRouterKey
		cfg.Providers[domain.ProviderOpenRouter] = p
		cfg.ActiveProvider = domain.ProviderOpenRouter
	}
	return cfg, nil
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

func (k *KeyringVault) SaveOpenRouterKey(key string) error {
	return keyring.Set(ServiceName, "openrouter_key", key)
}

func (k *KeyringVault) GetOpenRouterKey() (string, error) {
	key, err := keyring.Get(ServiceName, "openrouter_key")
	if err != nil || key == "" {
		return "", ErrCredentialsNotFound
	}
	return key, nil
}

func (k *KeyringVault) SaveAIConfig(cfg domain.AIConfig) error {
	bytes, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	if p, ok := cfg.Providers[domain.ProviderOpenRouter]; ok && p.APIKey != "" {
		_ = keyring.Set(ServiceName, "openrouter_key", p.APIKey)
	}
	return keyring.Set(ServiceName, "ai_config", string(bytes))
}

func (k *KeyringVault) GetAIConfig() (domain.AIConfig, error) {
	data, err := keyring.Get(ServiceName, "ai_config")
	if err == nil && data != "" {
		var cfg domain.AIConfig
		if err := json.Unmarshal([]byte(data), &cfg); err == nil {
			return cfg, nil
		}
	}
	legacyKey, errLegacy := keyring.Get(ServiceName, "openrouter_key")
	if errLegacy == nil && legacyKey != "" {
		cfg := domain.DefaultAIConfig()
		p := cfg.Providers[domain.ProviderOpenRouter]
		p.APIKey = legacyKey
		cfg.Providers[domain.ProviderOpenRouter] = p
		return cfg, nil
	}
	return domain.DefaultAIConfig(), ErrCredentialsNotFound
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

func (e *EnvVault) SaveOpenRouterKey(key string) error {
	return errors.New("no se admite escritura en variables de entorno")
}

func (e *EnvVault) GetOpenRouterKey() (string, error) {
	key := os.Getenv("OPENROUTER_API_KEY")
	if key == "" {
		return "", ErrCredentialsNotFound
	}
	return key, nil
}

func (e *EnvVault) SaveAIConfig(cfg domain.AIConfig) error {
	return errors.New("no se admite escritura en variables de entorno")
}

func (e *EnvVault) GetAIConfig() (domain.AIConfig, error) {
	cfg := domain.DefaultAIConfig()
	hasAny := false

	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		p := cfg.Providers[domain.ProviderAnthropic]
		p.APIKey = key
		if m := os.Getenv("ANTHROPIC_MODEL"); m != "" {
			p.DefaultModel = m
		}
		cfg.Providers[domain.ProviderAnthropic] = p
		cfg.ActiveProvider = domain.ProviderAnthropic
		cfg.ActiveModel = p.DefaultModel
		hasAny = true
	}

	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		p := cfg.Providers[domain.ProviderOpenAI]
		p.APIKey = key
		if m := os.Getenv("OPENAI_MODEL"); m != "" {
			p.DefaultModel = m
		}
		cfg.Providers[domain.ProviderOpenAI] = p
		if !hasAny {
			cfg.ActiveProvider = domain.ProviderOpenAI
			cfg.ActiveModel = p.DefaultModel
		}
		hasAny = true
	}

	if key := os.Getenv("OPENROUTER_API_KEY"); key != "" {
		p := cfg.Providers[domain.ProviderOpenRouter]
		p.APIKey = key
		if m := os.Getenv("OPENROUTER_MODEL"); m != "" {
			p.DefaultModel = m
		}
		cfg.Providers[domain.ProviderOpenRouter] = p
		if !hasAny {
			cfg.ActiveProvider = domain.ProviderOpenRouter
			cfg.ActiveModel = p.DefaultModel
		}
		hasAny = true
	}

	if ep := os.Getenv("OLLAMA_ENDPOINT"); ep != "" {
		p := cfg.Providers[domain.ProviderOllama]
		p.Endpoint = ep
		if m := os.Getenv("OLLAMA_MODEL"); m != "" {
			p.DefaultModel = m
		}
		cfg.Providers[domain.ProviderOllama] = p
		hasAny = true
	}

	if active := os.Getenv("SOLV_AI_PROVIDER"); active != "" {
		cfg.ActiveProvider = domain.AIProvider(active)
		if p, ok := cfg.Providers[cfg.ActiveProvider]; ok && p.DefaultModel != "" {
			cfg.ActiveModel = p.DefaultModel
		}
	}
	if activeM := os.Getenv("SOLV_AI_MODEL"); activeM != "" {
		cfg.ActiveModel = activeM
	}

	if !hasAny {
		return cfg, ErrCredentialsNotFound
	}
	return cfg, nil
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

func (c *CascadeVault) SaveOpenRouterKey(key string) error {
	err := c.keyring.SaveOpenRouterKey(key)
	if err == nil {
		return nil
	}
	return c.fileVault.SaveOpenRouterKey(key)
}

func (c *CascadeVault) GetOpenRouterKey() (string, error) {
	if key, err := c.keyring.GetOpenRouterKey(); err == nil && key != "" {
		return key, nil
	}
	if key, err := c.fileVault.GetOpenRouterKey(); err == nil && key != "" {
		return key, nil
	}
	if key, err := c.envVault.GetOpenRouterKey(); err == nil && key != "" {
		return key, nil
	}
	return "", ErrCredentialsNotFound
}

func (c *CascadeVault) SaveAIConfig(cfg domain.AIConfig) error {
	err := c.keyring.SaveAIConfig(cfg)
	if err == nil {
		return nil
	}
	return c.fileVault.SaveAIConfig(cfg)
}

func (c *CascadeVault) GetAIConfig() (domain.AIConfig, error) {
	if cfg, err := c.keyring.GetAIConfig(); err == nil && cfg.ActiveProvider != "" {
		return cfg, nil
	}
	if cfg, err := c.fileVault.GetAIConfig(); err == nil && cfg.ActiveProvider != "" {
		return cfg, nil
	}
	if cfg, err := c.envVault.GetAIConfig(); err == nil && cfg.ActiveProvider != "" {
		return cfg, nil
	}
	return domain.DefaultAIConfig(), ErrCredentialsNotFound
}

