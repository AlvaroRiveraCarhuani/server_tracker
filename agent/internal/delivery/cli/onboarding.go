package cli

import (
	"fmt"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/ports"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

var (
	brandStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#1e1e2e")).
			Background(lipgloss.Color("#b4befe")).
			Padding(0, 1)

	successStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#a6e3a1"))
)

// RunOnboarding lanza el formulario interactivo para registrar credenciales en la bóveda segura.
func RunOnboarding(vault ports.VaultPort) error {
	var serverURL string
	var secretToken string

	fmt.Println()
	fmt.Println(brandStyle.Render("🔐  SOLV SERVER TRACKER — Onboarding Seguro"))
	fmt.Println()

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("1. ¿Cuál es la URL de tu Servidor FastAPI (Control Plane)?").
				Placeholder("ej. http://192.168.1.100:8000").
				Value(&serverURL).
				Validate(func(s string) error {
					if len(s) == 0 {
						return fmt.Errorf("la URL del servidor no puede estar vacía")
					}
					return nil
				}),

			huh.NewInput().
				Title("2. Ingresa tu Token Secreto (Pre-Shared Key para HMAC)").
				Placeholder("El texto será ocultado por seguridad...").
				EchoMode(huh.EchoModePassword).
				Value(&secretToken).
				Validate(func(s string) error {
					if len(s) < 8 {
						return fmt.Errorf("el token debe tener al menos 8 caracteres")
					}
					return nil
				}),
		),
	)

	err := form.Run()
	if err != nil {
		return fmt.Errorf("onboarding cancelado: %w", err)
	}

	err = vault.Save(serverURL, secretToken)
	if err != nil {
		return fmt.Errorf("error guardando credenciales en la bóveda: %w", err)
	}

	fmt.Println()
	fmt.Println(successStyle.Render("✅ Configuración guardada y protegida criptográficamente."))
	fmt.Printf("🔧 Servidor Destino: %s\n", serverURL)
	fmt.Println("🔒 Las credenciales están blindadas sin archivos .env en texto plano.")

	return nil
}

// EnsureCredentials verifica si existen credenciales guardadas; si no, lanza el onboarding.
func EnsureCredentials(vault ports.VaultPort) (serverURL, secretToken string, err error) {
	serverURL, secretToken, err = vault.Get()
	if err == nil && serverURL != "" && secretToken != "" {
		return serverURL, secretToken, nil
	}

	fmt.Println("⚠️ No se detectaron credenciales previas en el sistema. Iniciando asistente...")
	err = RunOnboarding(vault)
	if err != nil {
		return "", "", err
	}

	return vault.Get()
}
