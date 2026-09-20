// Command colorpicker-preview runs just the part-color picker UI on its
// own, to try it out before it's wired into the real custom-theme
// assembly screen. Not part of the released carty binary.
package main

import (
	"fmt"
	"os"

	"github.com/dvdsvds/carty/internal/tui"
)

func main() {
	if err := tui.RunColorPickerPreview(); err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
}
