package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Technology contiene los metadatos y glifos de una tecnología.
type Technology struct {
	ID        string
	Name      string
	NerdGlyph string
	Color     lipgloss.Color
	Category  string
}

// Badge renderiza una pastilla/etiqueta tipográfica con estilo Catppuccin.
func (t Technology) Badge() string {
	glyphStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorBase).
		Background(t.Color).
		Padding(0, 1)

	nameStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorText).
		Background(ColorSurface0).
		Padding(0, 1)

	return fmt.Sprintf("%s%s", glyphStyle.Render(t.NerdGlyph), nameStyle.Render(" "+t.Name))
}

// Definition define los matchers y atributos de una tecnología.
type Definition struct {
	ID        string
	Name      string
	NerdGlyph string
	Color     lipgloss.Color
	Category  string
	Matchers  []string
}

// TechnologyRegistry administra el catálogo desacoplado de tecnologías.
type TechnologyRegistry struct {
	definitions []Definition
}

// NewTechnologyRegistry inicializa el catálogo de tecnologías.
func NewTechnologyRegistry() *TechnologyRegistry {
	return &TechnologyRegistry{
		definitions: []Definition{
			{
				ID:        "postgresql",
				Name:      "PostgreSQL",
				NerdGlyph: "󱤓",
				Color:     ColorBlue,
				Category:  "Database",
				Matchers:  []string{"postgres", "postgresql", "pgsql", "timescale"},
			},
			{
				ID:        "docker",
				Name:      "Docker",
				NerdGlyph: "󰡨",
				Color:     ColorBlue,
				Category:  "Runtime",
				Matchers:  []string{"docker", "dind"},
			},
			{
				ID:        "redis",
				Name:      "Redis",
				NerdGlyph: "",
				Color:     ColorRed,
				Category:  "Cache",
				Matchers:  []string{"redis", "valkey", "dragonfly"},
			},
			{
				ID:        "nginx",
				Name:      "NGINX",
				NerdGlyph: "",
				Color:     ColorGreen,
				Category:  "Proxy",
				Matchers:  []string{"nginx"},
			},
			{
				ID:        "traefik",
				Name:      "Traefik",
				NerdGlyph: "󱡠",
				Color:     ColorTeal,
				Category:  "Gateway",
				Matchers:  []string{"traefik"},
			},
			{
				ID:        "mysql",
				Name:      "MySQL",
				NerdGlyph: "",
				Color:     ColorPeach,
				Category:  "Database",
				Matchers:  []string{"mysql", "mariadb"},
			},
			{
				ID:        "mongodb",
				Name:      "MongoDB",
				NerdGlyph: "",
				Color:     ColorGreen,
				Category:  "Database",
				Matchers:  []string{"mongo", "mongodb"},
			},
			{
				ID:        "node",
				Name:      "Node.js",
				NerdGlyph: "",
				Color:     ColorGreen,
				Category:  "App",
				Matchers:  []string{"node", "nodejs", "next", "express"},
			},
			{
				ID:        "python",
				Name:      "Python",
				NerdGlyph: "",
				Color:     ColorYellow,
				Category:  "App",
				Matchers:  []string{"python", "fastapi", "django", "flask"},
			},
			{
				ID:        "go",
				Name:      "Go",
				NerdGlyph: "",
				Color:     ColorBlue,
				Category:  "App",
				Matchers:  []string{"golang", "go-", "solv-agent"},
			},
		},
	}
}

// Resolve busca una coincidencia en el catálogo o genera un fallback limpio.
func (r *TechnologyRegistry) Resolve(imageName, containerName string) Technology {
	target := strings.ToLower(imageName + " " + containerName)

	for _, def := range r.definitions {
		for _, m := range def.Matchers {
			if strings.Contains(target, m) {
				return Technology{
					ID:        def.ID,
					Name:      def.Name,
					NerdGlyph: def.NerdGlyph,
					Color:     def.Color,
					Category:  def.Category,
				}
			}
		}
	}

	// Fallback para servicios propios o no catalogados
	tag := containerName
	if tag == "" {
		parts := strings.Split(imageName, "/")
		tag = parts[len(parts)-1]
		tag = strings.Split(tag, ":")[0]
	}
	if len(tag) > 16 {
		tag = tag[:16]
	}

	return Technology{
		ID:        "custom",
		Name:      strings.ToUpper(tag),
		NerdGlyph: "󰡨",
		Color:     ColorMauve,
		Category:  "Service",
	}
}

var defaultRegistry = NewTechnologyRegistry()

// DetectTechnology busca en el registro global por conveniencia de la UI.
func DetectTechnology(imageName, containerName string) Technology {
	return defaultRegistry.Resolve(imageName, containerName)
}
