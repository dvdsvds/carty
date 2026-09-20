package tui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// colorPicker is a Paint-style 2D color grid: hue across the columns,
// saturation down the rows, at a fixed high value/brightness so every
// cell is vivid. Navigated with hjkl/arrows, like any other list cursor
// in this TUI.
type colorPicker struct {
	hueIndex int
	satIndex int
}

const (
	pickerHueSteps = 32 // columns
	pickerSatSteps = 12 // rows
	pickerValue    = 0.95
)

func newColorPicker() colorPicker {
	return colorPicker{hueIndex: 0, satIndex: 0}
}

func (p colorPicker) hex() string {
	hue := float64(p.hueIndex) / float64(pickerHueSteps) * 360
	sat := 1 - float64(p.satIndex)/float64(pickerSatSteps-1)
	return hsvToHex(hue, sat, pickerValue)
}

func (p *colorPicker) move(dx, dy int) {
	p.hueIndex = (p.hueIndex + dx + pickerHueSteps) % pickerHueSteps
	p.satIndex = clampInt(p.satIndex+dy, 0, pickerSatSteps-1)
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// hsvToHex converts h in [0,360), s and v in [0,1] to a "#rrggbb" hex
// string.
func hsvToHex(h, s, v float64) string {
	c := v * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := v - c

	var r, g, b float64
	switch {
	case h < 60:
		r, g, b = c, x, 0
	case h < 120:
		r, g, b = x, c, 0
	case h < 180:
		r, g, b = 0, c, x
	case h < 240:
		r, g, b = 0, x, c
	case h < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}

	to255 := func(v float64) int { return int(math.Round((v + m) * 255)) }
	return fmt.Sprintf("#%02x%02x%02x", to255(r), to255(g), to255(b))
}

// contrastMarkerColor picks black or white, whichever reads more clearly
// against the given "#rrggbb" background, via the standard relative
// luminance formula (same one used for WCAG contrast checks).
func contrastMarkerColor(hex string) string {
	var r, g, b int
	fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b)
	lum := 0.2126*float64(r) + 0.7152*float64(g) + 0.0722*float64(b)
	if lum > 140 {
		return "#111111"
	}
	return "#f2f2f2"
}

// renderColorPicker draws the grid, a highlighted cursor cell, and a
// swatch + hex readout of the currently selected color underneath.
func renderColorPicker(p colorPicker, label string) string {
	var b strings.Builder

	for row := 0; row < pickerSatSteps; row++ {
		for col := 0; col < pickerHueSteps; col++ {
			hue := float64(col) / float64(pickerHueSteps) * 360
			sat := 1 - float64(row)/float64(pickerSatSteps-1)
			hex := hsvToHex(hue, sat, pickerValue)

			cell := "  "
			style := lipgloss.NewStyle().Background(lipgloss.Color(hex))
			if row == p.satIndex && col == p.hueIndex {
				// A thin marker (one cell of the two), colored for
				// contrast against this specific background, so the
				// cursor stays legible without visually blotting out
				// light cells (a near-black diamond over a pastel cell
				// used to read as "this cell is just black").
				cell = "●" + " "
				style = style.Foreground(lipgloss.Color(contrastMarkerColor(hex)))
			}
			b.WriteString(style.Render(cell))
		}
		b.WriteString("\n")
	}

	hex := p.hex()
	swatch := lipgloss.NewStyle().Background(lipgloss.Color(hex)).Render("    ")
	b.WriteString("\n")
	if label != "" {
		b.WriteString(mutedStyle.Render(label + ": "))
	}
	b.WriteString(swatch + " " + lipgloss.NewStyle().Foreground(lipgloss.Color(hex)).Bold(true).Render(hex))

	return b.String()
}
