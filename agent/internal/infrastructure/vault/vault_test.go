package vault_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alvaroriverac/server_tracker_agent/internal/infrastructure/vault"
)

func TestVault_EncryptDecryptLocalFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "solv_vault_test")
	if err != nil {
		t.Fatalf("error creando temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	filePath := filepath.Join(tempDir, "vault.enc")
	passphrase := "super_secret_master_key_123"

	v := vault.NewFileVault(filePath, passphrase)

	serverURL := "https://tracker.solv.internal:8000"
	secretToken := "psk_live_9876543210abcdef"

	// 1. Guardar credenciales cifradas
	err = v.Save(serverURL, secretToken)
	if err != nil {
		t.Fatalf("error al guardar en la bóveda: %v", err)
	}

	// 2. Verificar permisos 0600 en el archivo (D2)
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("error leyendo archivo de bóveda: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("los permisos deben ser estrictamente 0600, obtenido %v", info.Mode().Perm())
	}

	// 3. Recuperar credenciales con la misma passphrase
	gotURL, gotToken, err := v.Get()
	if err != nil {
		t.Fatalf("error al leer de la bóveda: %v", err)
	}

	if gotURL != serverURL {
		t.Errorf("esperado URL %s, obtenido %s", serverURL, gotURL)
	}
	if gotToken != secretToken {
		t.Errorf("esperado Token %s, obtenido %s", secretToken, gotToken)
	}

	// 4. Intentar descifrar con passphrase incorrecta -> debe fallar
	vBad := vault.NewFileVault(filePath, "wrong_password")
	_, _, err = vBad.Get()
	if err == nil {
		t.Fatalf("esperado error de autenticación GCM con password incorrecta, pero tuvo éxito")
	}
}

func TestVault_CascadeFallback_EnvVars(t *testing.T) {
	os.Setenv("SOLV_SERVER_URL", "http://ci-env:8000")
	os.Setenv("SOLV_AGENT_SECRET", "ci_token_xyz")
	defer func() {
		os.Unsetenv("SOLV_SERVER_URL")
		os.Unsetenv("SOLV_AGENT_SECRET")
	}()

	v := vault.NewEnvVault()
	url, token, err := v.Get()
	if err != nil {
		t.Fatalf("error leyendo variables de entorno: %v", err)
	}
	if url != "http://ci-env:8000" || token != "ci_token_xyz" {
		t.Fatalf("valores de env incorrectos")
	}
}
