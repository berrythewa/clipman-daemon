package theme

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// CustomTheme extends the default Fyne theme
type CustomTheme struct {
	fyne.Theme
}

// NewCustomTheme creates a new custom theme
func NewCustomTheme() *CustomTheme {
	return &CustomTheme{
		Theme: theme.DefaultTheme(),
	}
}

// Icon returns a custom icon for the given name
func (t *CustomTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	switch name {
	case "settings":
		return theme.SettingsIcon()
	case "pairing":
		return theme.ComputerIcon()
	case "clear":
		return theme.DeleteIcon()
	case "document":
		return theme.DocumentIcon()
	default:
		return t.Theme.Icon(name)
	}
}

// Color returns a custom color for the given name
func (t *CustomTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case "primary":
		return color.NRGBA{R: 0, G: 120, B: 212, A: 255}
	case "secondary":
		return color.NRGBA{R: 0, G: 99, B: 177, A: 255}
	default:
		return t.Theme.Color(name, variant)
	}
}

// Size returns a custom size for the given name
func (t *CustomTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case "padding":
		return 8
	case "scrollBarSize":
		return 8
	default:
		return t.Theme.Size(name)
	}
} 