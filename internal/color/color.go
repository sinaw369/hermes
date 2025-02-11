package color

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"math"
)

// Color represents an RGB color with a name and hex code
type Color struct {
	Name string
	Hex  string
}

// NewColors returns a structured color map for easy access
func NewColors() struct {
	// Warm Colors (Sunset & Fire)
	SunsetOrange Color
	CoralRed     Color
	BurntOrange  Color
	AmberGlow    Color
	GoldenSunset Color

	// Cool Colors (Nature & Water)
	ForestGreen Color
	Emerald     Color
	Turquoise   Color
	DeepSeaBlue Color
	OceanBlue   Color

	// Pastel & Soft Colors
	LavenderMist Color
	BlushPink    Color
	PeachFuzz    Color
	MintGreen    Color
	BabyBlue     Color

	// Vibrant & Neon Colors
	NeonMagenta     Color
	ElectricBlue    Color
	LimeGreen       Color
	UltraViolet     Color
	FluorescentCyan Color

	// Dark & Elegant Colors
	MidnightBlue Color
	CharcoalGrey Color
	DeepPlum     Color
	Burgundy     Color
	OnyxBlack    Color
} {
	return struct {
		// Warm Colors (Sunset & Fire)
		SunsetOrange Color
		CoralRed     Color
		BurntOrange  Color
		AmberGlow    Color
		GoldenSunset Color

		// Cool Colors (Nature & Water)
		ForestGreen Color
		Emerald     Color
		Turquoise   Color
		DeepSeaBlue Color
		OceanBlue   Color

		// Pastel & Soft Colors
		LavenderMist Color
		BlushPink    Color
		PeachFuzz    Color
		MintGreen    Color
		BabyBlue     Color

		// Vibrant & Neon Colors
		NeonMagenta     Color
		ElectricBlue    Color
		LimeGreen       Color
		UltraViolet     Color
		FluorescentCyan Color

		// Dark & Elegant Colors
		MidnightBlue Color
		CharcoalGrey Color
		DeepPlum     Color
		Burgundy     Color
		OnyxBlack    Color
	}{
		// Warm Colors (Sunset & Fire)
		SunsetOrange: Color{"Sunset Orange", "#FF5733"},
		CoralRed:     Color{"Coral Red", "#FF6F61"},
		BurntOrange:  Color{"Burnt Orange", "#CC5500"},
		AmberGlow:    Color{"Amber Glow", "#FFBF00"},
		GoldenSunset: Color{"Golden Sunset", "#FFC300"},

		// Cool Colors (Nature & Water)
		ForestGreen: Color{"Forest Green", "#228B22"},
		Emerald:     Color{"Emerald", "#50C878"},
		Turquoise:   Color{"Turquoise", "#40E0D0"},
		DeepSeaBlue: Color{"Deep Sea Blue", "#005F6B"},
		OceanBlue:   Color{"Ocean Blue", "#0077BE"},

		// Pastel & Soft Colors
		LavenderMist: Color{"Lavender Mist", "#E6E6FA"},
		BlushPink:    Color{"Blush Pink", "#FFB6C1"},
		PeachFuzz:    Color{"Peach Fuzz", "#FFDAB9"},
		MintGreen:    Color{"Mint Green", "#98FB98"},
		BabyBlue:     Color{"Baby Blue", "#89CFF0"},

		// Vibrant & Neon Colors
		NeonMagenta:     Color{"Neon Magenta", "#FF1493"},
		ElectricBlue:    Color{"Electric Blue", "#7DF9FF"},
		LimeGreen:       Color{"Lime Green", "#32CD32"},
		UltraViolet:     Color{"Ultra Violet", "#645394"},
		FluorescentCyan: Color{"Fluorescent Cyan", "#00FFFF"},

		// Dark & Elegant Colors
		MidnightBlue: Color{"Midnight Blue", "#191970"},
		CharcoalGrey: Color{"Charcoal Grey", "#36454F"},
		DeepPlum:     Color{"Deep Plum", "#673147"},
		Burgundy:     Color{"Burgundy", "#800020"},
		OnyxBlack:    Color{"Onyx Black", "#353839"},
	}
}

// ColorRGB represents RGB values
type ColorRGB struct {
	R, G, B int
}

// Converts a hex color to RGB format
func hexToRGB(hex string) ColorRGB {
	var r, g, b int
	fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b)
	return ColorRGB{r, g, b}
}

// Converts an RGB color back to a hex string
func rgbToHex(c ColorRGB) string {
	return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B)
}

// Interpolates between two RGB colors for smooth gradients
func interpolateColor(c1, c2 ColorRGB, t float64) ColorRGB {
	return ColorRGB{
		R: int(math.Round(float64(c1.R) + (float64(c2.R)-float64(c1.R))*t)),
		G: int(math.Round(float64(c1.G) + (float64(c2.G)-float64(c1.G))*t)),
		B: int(math.Round(float64(c1.B) + (float64(c2.B)-float64(c1.B))*t)),
	}
}

// Generates a gradient effect over a text string
func GradientString(text, startHex, endHex string) string {
	startColor := hexToRGB(startHex)
	endColor := hexToRGB(endHex)

	length := len(text)
	if length == 0 {
		return ""
	}

	var gradientText string
	for i, char := range text {
		t := float64(i) / float64(length-1) // Normalize the position
		color := interpolateColor(startColor, endColor, t)
		hexColor := rgbToHex(color)

		// Apply Lipgloss styling
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(hexColor))
		gradientText += style.Render(string(char))
	}

	return gradientText
}
