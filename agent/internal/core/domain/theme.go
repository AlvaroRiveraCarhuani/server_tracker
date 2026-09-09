package domain

// ThemeConfig almacena las preferencias estéticas del operador para la TUI.
type ThemeConfig struct {
	ActiveTheme string `json:"active_theme"` // "tokyo-night", "catppuccin", "gruvbox", "nord", "oled"
	NerdFonts   bool   `json:"nerd_fonts"`   // true para glifos ricos, false para ASCII puro
	BorderStyle string `json:"border_style"` // "double", "rounded", "sharp"
}

// DefaultThemeConfig devuelve la configuración estética predeterminada.
func DefaultThemeConfig() ThemeConfig {
	return ThemeConfig{
		ActiveTheme: "tokyo-night",
		NerdFonts:   true,
		BorderStyle: "double",
	}
}

// ThemeMetadata describe una opción de tema en el catálogo.
type ThemeMetadata struct {
	ID          string
	Name        string
	Description string
	PreviewHex  []string
	DefaultBorder string
}

// AvailableThemes lista las paletas soportadas por el motor de diseño.
var AvailableThemes = []ThemeMetadata{
	{
		ID:            "tokyo-night",
		Name:          "Tokyo Night",
		Description:   "Azul noche profundo con acentos neón y bordes dobles (LazyVim)",
		PreviewHex:    []string{"#1a1b26", "#7aa2f7", "#bb9af7", "#7dcfff", "#9ece6a"},
		DefaultBorder: "double",
	},
	{
		ID:            "catppuccin",
		Name:          "Catppuccin Mocha",
		Description:   "Pastel suave de alto contraste con bordes redondeados",
		PreviewHex:    []string{"#1e1e2e", "#cba6f7", "#b4befe", "#a6e3a1", "#fab387"},
		DefaultBorder: "rounded",
	},
	{
		ID:            "gruvbox",
		Name:          "Gruvbox Dark",
		Description:   "Cálido vintage retro-hacker con bordes nítidos",
		PreviewHex:    []string{"#1d2021", "#ebdbb2", "#b8bb26", "#fabd2f", "#fe8019"},
		DefaultBorder: "sharp",
	},
	{
		ID:            "nord",
		Name:          "Nord Frost",
		Description:   "Minimalismo ártico sobrio con azules polares",
		PreviewHex:    []string{"#2e3440", "#88c0d0", "#81a1c1", "#a3be8c", "#ebcb8b"},
		DefaultBorder: "rounded",
	},
	{
		ID:            "vesper",
		Name:          "Vesper Dark",
		Description:   "Minimalismo negro carbón #101010 con acentos ámbar refinados",
		PreviewHex:    []string{"#101010", "#ffc799", "#e5a46b", "#ffffff", "#505050"},
		DefaultBorder: "double",
	},
	{
		ID:            "kanagawa-dragon",
		Name:          "Kanagawa Dragon",
		Description:   "Tinta japonesa sumi-e sobre carbón #181616, mate y cero fatiga",
		PreviewHex:    []string{"#181616", "#c8c093", "#87a987", "#8ba4b0", "#957fb8"},
		DefaultBorder: "rounded",
	},
	{
		ID:            "dracula-pro",
		Name:          "Dracula Pro Blade",
		Description:   "Negro gótico profundo #0f0f14 con morado, cian y rosa de alto contraste",
		PreviewHex:    []string{"#0f0f14", "#bd93f9", "#8be9fd", "#ff79c6", "#50fa7b"},
		DefaultBorder: "double",
	},
	{
		ID:            "phosphor",
		Name:          "Matrix Phosphor",
		Description:   "Negro absoluto #000000 con fósforo verde monocromático retro-terminal",
		PreviewHex:    []string{"#000000", "#00ff41", "#00ff66", "#33ff33", "#051a05"},
		DefaultBorder: "sharp",
	},
	{
		ID:            "oled",
		Name:          "OLED Pure Black",
		Description:   "Negro absoluto #000000 con acentos verdes y cian",
		PreviewHex:    []string{"#000000", "#00ff88", "#00ffff", "#ffffff", "#ff0055"},
		DefaultBorder: "double",
	},
}
