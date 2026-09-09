package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ThemePalette define los tokens cromáticos y estilo de borde de una paleta.
type ThemePalette struct {
	ID            string
	Name          string
	Base          lipgloss.Color
	Mantle        lipgloss.Color
	Crust         lipgloss.Color
	Surface0      lipgloss.Color
	Surface1      lipgloss.Color
	Surface2      lipgloss.Color
	Overlay0      lipgloss.Color
	Overlay2      lipgloss.Color
	Subtext0      lipgloss.Color
	Subtext1      lipgloss.Color
	Text          lipgloss.Color
	Lavender      lipgloss.Color
	Mauve         lipgloss.Color
	Green         lipgloss.Color
	Yellow        lipgloss.Color
	Peach         lipgloss.Color
	Red           lipgloss.Color
	Blue          lipgloss.Color
	Teal          lipgloss.Color
	DefaultBorder lipgloss.Border
}

var (
	TokyoNightPalette = ThemePalette{
		ID:            "tokyo-night",
		Name:          "Tokyo Night",
		Base:          lipgloss.Color("#1a1b26"),
		Mantle:        lipgloss.Color("#16161e"),
		Crust:         lipgloss.Color("#13141c"),
		Surface0:      lipgloss.Color("#24283b"),
		Surface1:      lipgloss.Color("#292e42"),
		Surface2:      lipgloss.Color("#414868"),
		Overlay0:      lipgloss.Color("#565f89"),
		Overlay2:      lipgloss.Color("#737aa2"),
		Subtext0:      lipgloss.Color("#7aa2f7"),
		Subtext1:      lipgloss.Color("#a9b1d6"),
		Text:          lipgloss.Color("#c0caf5"),
		Lavender:      lipgloss.Color("#7dcfff"),
		Mauve:         lipgloss.Color("#bb9af7"),
		Green:         lipgloss.Color("#9ece6a"),
		Yellow:        lipgloss.Color("#e0af68"),
		Peach:         lipgloss.Color("#ff9e64"),
		Red:           lipgloss.Color("#f7768e"),
		Blue:          lipgloss.Color("#7aa2f7"),
		Teal:          lipgloss.Color("#2ac3de"),
		DefaultBorder: lipgloss.DoubleBorder(),
	}

	CatppuccinMochaPalette = ThemePalette{
		ID:            "catppuccin",
		Name:          "Catppuccin Mocha",
		Base:          lipgloss.Color("#1e1e2e"),
		Mantle:        lipgloss.Color("#181825"),
		Crust:         lipgloss.Color("#11111b"),
		Surface0:      lipgloss.Color("#313244"),
		Surface1:      lipgloss.Color("#45475a"),
		Surface2:      lipgloss.Color("#585b70"),
		Overlay0:      lipgloss.Color("#6c7086"),
		Overlay2:      lipgloss.Color("#9399b2"),
		Subtext0:      lipgloss.Color("#a6adc8"),
		Subtext1:      lipgloss.Color("#bac2de"),
		Text:          lipgloss.Color("#cdd6f4"),
		Lavender:      lipgloss.Color("#b4befe"),
		Mauve:         lipgloss.Color("#cba6f7"),
		Green:         lipgloss.Color("#a6e3a1"),
		Yellow:        lipgloss.Color("#f9e2af"),
		Peach:         lipgloss.Color("#fab387"),
		Red:           lipgloss.Color("#f38ba8"),
		Blue:          lipgloss.Color("#89b4fa"),
		Teal:          lipgloss.Color("#94e2d5"),
		DefaultBorder: lipgloss.RoundedBorder(),
	}

	GruvboxDarkPalette = ThemePalette{
		ID:            "gruvbox",
		Name:          "Gruvbox Dark",
		Base:          lipgloss.Color("#1d2021"),
		Mantle:        lipgloss.Color("#18191a"),
		Crust:         lipgloss.Color("#141617"),
		Surface0:      lipgloss.Color("#282828"),
		Surface1:      lipgloss.Color("#3c3836"),
		Surface2:      lipgloss.Color("#504945"),
		Overlay0:      lipgloss.Color("#7c6f64"),
		Overlay2:      lipgloss.Color("#928374"),
		Subtext0:      lipgloss.Color("#d5c4a1"),
		Subtext1:      lipgloss.Color("#ebdbb2"),
		Text:          lipgloss.Color("#fbf1c7"),
		Lavender:      lipgloss.Color("#83a598"),
		Mauve:         lipgloss.Color("#d3869b"),
		Green:         lipgloss.Color("#b8bb26"),
		Yellow:        lipgloss.Color("#fabd2f"),
		Peach:         lipgloss.Color("#fe8019"),
		Red:           lipgloss.Color("#fb4934"),
		Blue:          lipgloss.Color("#83a598"),
		Teal:          lipgloss.Color("#8ec07c"),
		DefaultBorder: lipgloss.NormalBorder(),
	}

	NordFrostPalette = ThemePalette{
		ID:            "nord",
		Name:          "Nord Frost",
		Base:          lipgloss.Color("#2e3440"),
		Mantle:        lipgloss.Color("#272c36"),
		Crust:         lipgloss.Color("#22262e"),
		Surface0:      lipgloss.Color("#3b4252"),
		Surface1:      lipgloss.Color("#434c5e"),
		Surface2:      lipgloss.Color("#4c566a"),
		Overlay0:      lipgloss.Color("#616e88"),
		Overlay2:      lipgloss.Color("#7b88a1"),
		Subtext0:      lipgloss.Color("#81a1c1"),
		Subtext1:      lipgloss.Color("#d8dee9"),
		Text:          lipgloss.Color("#eceff4"),
		Lavender:      lipgloss.Color("#88c0d0"),
		Mauve:         lipgloss.Color("#b48ead"),
		Green:         lipgloss.Color("#a3be8c"),
		Yellow:        lipgloss.Color("#ebcb8b"),
		Peach:         lipgloss.Color("#d08770"),
		Red:           lipgloss.Color("#bf616a"),
		Blue:          lipgloss.Color("#5e81ac"),
		Teal:          lipgloss.Color("#8fbcbb"),
		DefaultBorder: lipgloss.RoundedBorder(),
	}

	OLEDPureBlackPalette = ThemePalette{
		ID:            "oled",
		Name:          "OLED Pure Black",
		Base:          lipgloss.Color("#000000"),
		Mantle:        lipgloss.Color("#000000"),
		Crust:         lipgloss.Color("#000000"),
		Surface0:      lipgloss.Color("#121212"),
		Surface1:      lipgloss.Color("#222222"),
		Surface2:      lipgloss.Color("#333333"),
		Overlay0:      lipgloss.Color("#555555"),
		Overlay2:      lipgloss.Color("#888888"),
		Subtext0:      lipgloss.Color("#00ffff"),
		Subtext1:      lipgloss.Color("#cccccc"),
		Text:          lipgloss.Color("#ffffff"),
		Lavender:      lipgloss.Color("#00ffff"),
		Mauve:         lipgloss.Color("#ff00ff"),
		Green:         lipgloss.Color("#00ff88"),
		Yellow:        lipgloss.Color("#ffff00"),
		Peach:         lipgloss.Color("#ff8800"),
		Red:           lipgloss.Color("#ff0055"),
		Blue:          lipgloss.Color("#00aaff"),
		Teal:          lipgloss.Color("#00ffcc"),
		DefaultBorder: lipgloss.DoubleBorder(),
	}
)

var ThemePalettes = map[string]ThemePalette{
	"tokyo-night": TokyoNightPalette,
	"catppuccin":  CatppuccinMochaPalette,
	"gruvbox":     GruvboxDarkPalette,
	"nord":        NordFrostPalette,
	"oled":        OLEDPureBlackPalette,
}

// Variables activas en tiempo de ejecución
var (
	CurrentPalette = TokyoNightPalette
	NerdFontsMode  = true

	ColorBase     = TokyoNightPalette.Base
	ColorMantle   = TokyoNightPalette.Mantle
	ColorCrust    = TokyoNightPalette.Crust
	ColorSurface0 = TokyoNightPalette.Surface0
	ColorSurface1 = TokyoNightPalette.Surface1
	ColorSurface2 = TokyoNightPalette.Surface2
	ColorOverlay0 = TokyoNightPalette.Overlay0
	ColorOverlay2 = TokyoNightPalette.Overlay2
	ColorSubtext0 = TokyoNightPalette.Subtext0
	ColorSubtext1 = TokyoNightPalette.Subtext1
	ColorText     = TokyoNightPalette.Text
	ColorLavender = TokyoNightPalette.Lavender
	ColorMauve    = TokyoNightPalette.Mauve
	ColorGreen    = TokyoNightPalette.Green
	ColorYellow   = TokyoNightPalette.Yellow
	ColorPeach    = TokyoNightPalette.Peach
	ColorRed      = TokyoNightPalette.Red
	ColorBlue     = TokyoNightPalette.Blue
	ColorTeal     = TokyoNightPalette.Teal

	CurrentBorder = lipgloss.DoubleBorder()

	StyleTitle             lipgloss.Style
	StyleSolvBranding      lipgloss.Style
	StyleCard              lipgloss.Style
	StyleCardTitle         lipgloss.Style
	StyleHeader            lipgloss.Style
	StyleRowFocus          lipgloss.Style
	StyleStatusRunning     lipgloss.Style
	StyleStatusStopped     lipgloss.Style
	StyleStatusPaused      lipgloss.Style
	StyleStatusCritical    lipgloss.Style
	StyleEgressNormal      lipgloss.Style
	StyleEgressMedium      lipgloss.Style
	StyleEgressWarm        lipgloss.Style
	StyleEgressDanger      lipgloss.Style
	StyleStatusBar         lipgloss.Style
	StyleFilterPrompt      lipgloss.Style
	StyleModal             lipgloss.Style
	StyleModalTitle        lipgloss.Style
	StyleBtnFocusedConfirm lipgloss.Style
	StyleBtnFocusedCancel  lipgloss.Style
	StyleBtnBlurred        lipgloss.Style
	StyleAIOpsBanner       lipgloss.Style
	StyleAIOpsTag          lipgloss.Style
	StyleSparklineNormal   lipgloss.Style
	StyleSparklineWarning  lipgloss.Style
	StyleSparklineDanger   lipgloss.Style
	StyleTrendsBox         lipgloss.Style
)

func init() {
	ApplyTheme("tokyo-night", "double", true)
}

// ApplyTheme recalcula todos los estilos Lipgloss en tiempo de ejecución.
func ApplyTheme(themeID, borderStyle string, nerdFonts bool) {
	p, ok := ThemePalettes[themeID]
	if !ok {
		p = TokyoNightPalette
	}
	CurrentPalette = p
	NerdFontsMode = nerdFonts

	ColorBase = p.Base
	ColorMantle = p.Mantle
	ColorCrust = p.Crust
	ColorSurface0 = p.Surface0
	ColorSurface1 = p.Surface1
	ColorSurface2 = p.Surface2
	ColorOverlay0 = p.Overlay0
	ColorOverlay2 = p.Overlay2
	ColorSubtext0 = p.Subtext0
	ColorSubtext1 = p.Subtext1
	ColorText = p.Text
	ColorLavender = p.Lavender
	ColorMauve = p.Mauve
	ColorGreen = p.Green
	ColorYellow = p.Yellow
	ColorPeach = p.Peach
	ColorRed = p.Red
	ColorBlue = p.Blue
	ColorTeal = p.Teal

	switch borderStyle {
	case "double":
		CurrentBorder = lipgloss.DoubleBorder()
	case "rounded":
		CurrentBorder = lipgloss.RoundedBorder()
	case "sharp", "normal":
		CurrentBorder = lipgloss.NormalBorder()
	default:
		CurrentBorder = p.DefaultBorder
	}

	StyleTitle = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorBase).
		Background(ColorLavender).
		Padding(0, 1)

	StyleSolvBranding = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorMauve)

	StyleCard = lipgloss.NewStyle().
		BorderStyle(CurrentBorder).
		BorderForeground(ColorSurface2).
		Padding(0, 1)

	StyleCardTitle = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorLavender)

	StyleHeader = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorMauve).
		BorderStyle(CurrentBorder).
		BorderBottom(true).
		BorderForeground(ColorSurface2)

	StyleRowFocus = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorText).
		Background(ColorSurface0)

	StyleStatusRunning = lipgloss.NewStyle().Foreground(ColorGreen).Bold(true)
	StyleStatusStopped = lipgloss.NewStyle().Foreground(ColorOverlay2)
	StyleStatusPaused = lipgloss.NewStyle().Foreground(ColorYellow).Bold(true)
	StyleStatusCritical = lipgloss.NewStyle().Foreground(ColorRed).Bold(true)

	StyleEgressNormal = lipgloss.NewStyle().Foreground(ColorSubtext0)
	StyleEgressMedium = lipgloss.NewStyle().Foreground(ColorText)
	StyleEgressWarm = lipgloss.NewStyle().Foreground(ColorPeach).Bold(true)
	StyleEgressDanger = lipgloss.NewStyle().Foreground(ColorRed).Bold(true)

	StyleStatusBar = lipgloss.NewStyle().
		Foreground(ColorSubtext1).
		Background(ColorMantle).
		Padding(0, 1)

	StyleFilterPrompt = lipgloss.NewStyle().
		Foreground(ColorMauve).
		Bold(true)

	StyleModal = lipgloss.NewStyle().
		BorderStyle(CurrentBorder).
		BorderForeground(ColorPeach).
		BorderBackground(ColorSurface0).
		Padding(1, 2).
		Background(ColorSurface0)

	StyleModalTitle = lipgloss.NewStyle().
		Bold(true).
		Background(ColorSurface0).
		Foreground(ColorPeach)

	StyleBtnFocusedConfirm = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorBase).
		Background(ColorGreen).
		Padding(0, 1)

	StyleBtnFocusedCancel = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorBase).
		Background(ColorPeach).
		Padding(0, 1)

	StyleBtnBlurred = lipgloss.NewStyle().
		Foreground(ColorSubtext0).
		Padding(0, 1)

	StyleAIOpsBanner = lipgloss.NewStyle().
		BorderStyle(CurrentBorder).
		BorderTop(true).
		BorderBottom(true).
		BorderForeground(ColorSurface1).
		Foreground(ColorSubtext1).
		Padding(0, 1)

	StyleAIOpsTag = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorMauve)

	StyleSparklineNormal = lipgloss.NewStyle().Foreground(ColorGreen)
	StyleSparklineWarning = lipgloss.NewStyle().Foreground(ColorPeach)
	StyleSparklineDanger = lipgloss.NewStyle().Foreground(ColorRed)

	StyleTrendsBox = lipgloss.NewStyle().
		BorderStyle(CurrentBorder).
		BorderForeground(ColorSurface1).
		Padding(0, 1).
		MarginBottom(1)
}

// FormatStatus renderiza los glifos con semántica cromática estricta y soporte para Nerd Fonts o ASCII.
func FormatStatus(status string, ramBytes, ramLimit uint64) (glyph, text string, style lipgloss.Style) {
	isOOM := false
	if ramLimit > 0 && status == "running" {
		if float64(ramBytes)/float64(ramLimit) >= 0.85 {
			isOOM = true
		}
	}

	if isOOM {
		if NerdFontsMode {
			return "󰀪", "OOM_RISK", StyleStatusCritical
		}
		return "[!!]", "OOM_RISK", StyleStatusCritical
	}

	switch strings.ToLower(status) {
	case "running":
		if NerdFontsMode {
			return "󰄴", "RUNNING", StyleStatusRunning
		}
		return "[OK]", "RUNNING", StyleStatusRunning
	case "exited", "stopped":
		if NerdFontsMode {
			return "󰅖", "STOPPED", StyleStatusStopped
		}
		return "[--]", "STOPPED", StyleStatusStopped
	case "paused":
		if NerdFontsMode {
			return "󰏤", "PAUSED", StyleStatusPaused
		}
		return "[||]", "PAUSED", StyleStatusPaused
	case "restarting", "dead":
		if NerdFontsMode {
			return "󰀪", strings.ToUpper(status), StyleStatusCritical
		}
		return "[!!]", strings.ToUpper(status), StyleStatusCritical
	default:
		if NerdFontsMode {
			return "󰋼", strings.ToUpper(status), StyleStatusStopped
		}
		return "[?]", strings.ToUpper(status), StyleStatusStopped
	}
}

// FormatEgress aplica el semáforo financiero y alineación de red
func FormatEgress(bytesSec float64) (formatted string, style lipgloss.Style) {
	kbSec := bytesSec / 1024.0
	mbSec := kbSec / 1024.0

	var numStr string
	if mbSec >= 1.0 {
		numStr = fmt.Sprintf("%.2f MB/s", mbSec)
	} else {
		numStr = fmt.Sprintf("%.1f KB/s", kbSec)
	}

	if bytesSec < 500*1024 { // < 500 KB/s
		return numStr, StyleEgressNormal
	} else if bytesSec < 5*1024*1024 { // 500 KB/s - 5 MB/s
		return numStr, StyleEgressMedium
	} else if bytesSec < 50*1024*1024 { // 5 MB/s - 50 MB/s
		return numStr, StyleEgressWarm
	}
	// > 50 MB/s (Riesgo Financiero)
	return numStr, StyleEgressDanger
}
