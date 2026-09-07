package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/alvaroriverac/server_tracker_agent/internal/delivery/cli"
	"github.com/alvaroriverac/server_tracker_agent/internal/delivery/tui"
	"github.com/alvaroriverac/server_tracker_agent/internal/infrastructure/buffer"
	"github.com/alvaroriverac/server_tracker_agent/internal/infrastructure/docker"
	"github.com/alvaroriverac/server_tracker_agent/internal/infrastructure/transport"
	"github.com/alvaroriverac/server_tracker_agent/internal/infrastructure/vault"
)

func main() {
	mode := flag.String("mode", "", "Modo de ejecución: 'daemon', 'tui', 'onboarding', 'status'")
	interval := flag.Duration("interval", 10*time.Second, "Intervalo de muestreo para el modo daemon")
	flag.Parse()

	// Ruta por defecto para la bóveda local en caso de fallback
	homeDir, _ := os.UserHomeDir()
	vaultPath := filepath.Join(homeDir, ".solv", "vault.enc")
	passphrase := os.Getenv("SOLV_VAULT_PASSPHRASE")
	if passphrase == "" {
		passphrase = "solv_default_host_entropy"
	}

	vaultService := vault.NewCascadeVault(vaultPath, passphrase)

	collector, err := docker.NewDockerCollector()
	if err != nil {
		log.Fatalf("❌ Error conectando con el socket de Docker (/var/run/docker.sock): %v\nVerifica que el servicio Docker esté corriendo y tu usuario pertenezca al grupo docker.", err)
	}

	selectedMode := *mode
	if selectedMode == "" && len(flag.Args()) > 0 {
		selectedMode = flag.Args()[0]
	}

	switch selectedMode {
	case "onboarding":
		if err := cli.RunOnboarding(vaultService); err != nil {
			log.Fatalf("❌ Error en onboarding: %v", err)
		}

	case "tui":
		if err := tui.RunTUI(collector); err != nil {
			log.Fatalf("❌ Error ejecutando TUI: %v", err)
		}

	case "daemon":
		serverURL, secretToken, err := cli.EnsureCredentials(vaultService)
		if err != nil {
			log.Fatalf("❌ No se pudieron cargar las credenciales: %v", err)
		}

		ringBuffer := buffer.NewRingBuffer(1000)
		httpTransport := transport.NewHTTPTransportClient(serverURL, secretToken)

		if err := cli.RunDaemon(collector, vaultService, ringBuffer, httpTransport, *interval); err != nil {
			log.Fatalf("❌ Error en daemon: %v", err)
		}

	case "status":
		serverURL, _, err := vaultService.Get()
		if err != nil {
			fmt.Println("❌ Estado: Sin credenciales configuradas.")
		} else {
			fmt.Printf("✅ Estado: Configurado y conectado a %s\n", serverURL)
		}

	default:
		// Si no se especifica modo, abrimos la TUI interactiva si hay TTY, o mostramos ayuda
		fmt.Println("🛰️  SOLV Server Tracker — Agente Host")
		fmt.Println("Uso: solv-agent [opciones]")
		fmt.Println("  --mode=tui          Lanza la interfaz de terminal interactiva")
		fmt.Println("  --mode=daemon       Lanza el recolector en segundo plano")
		fmt.Println("  --mode=onboarding   Configura credenciales interactivas")
		fmt.Println("  --mode=status       Verifica el estado de configuración")
		fmt.Println()
		fmt.Println("Iniciando TUI por defecto...")
		if err := tui.RunTUI(collector); err != nil {
			log.Fatalf("❌ Error ejecutando TUI: %v", err)
		}
	}
}
