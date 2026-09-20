package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// pickerPreviewModel is a throwaway bubbletea model for trying the color
// picker on its own, via cmd/colorpicker-preview. Not part of the real
// app flow.
type pickerPreviewModel struct {
	picker colorPicker
}

func (m pickerPreviewModel) Init() tea.Cmd { return nil }

func (m pickerPreviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch k.String() {
	case "q", "ctrl+c", "enter":
		return m, tea.Quit
	case "left", "h":
		m.picker.move(-1, 0)
	case "right", "l":
		m.picker.move(1, 0)
	case "up", "k":
		m.picker.move(0, -1)
	case "down", "j":
		m.picker.move(0, 1)
	}
	return m, nil
}

func (m pickerPreviewModel) View() string {
	return "색상 피커 미리보기 — hjkl/방향키로 이동, enter/q로 종료\n\n" +
		renderColorPicker(m.picker, "선택된 색") + "\n"
}

// RunColorPickerPreview launches a minimal standalone program showing just
// the color picker, for trying it out before it's wired into the real
// part-assembly screen.
func RunColorPickerPreview() error {
	p := tea.NewProgram(pickerPreviewModel{picker: newColorPicker()})
	_, err := p.Run()
	return err
}
