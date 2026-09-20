package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dvdsvds/carty/internal/catalog"
)

// assembledPart is one part in a "compose" category's assembled
// sequence, with the color the user picked for it. Picker (not a bare
// hex string) is kept so re-opening the color editor for an already-set
// part starts the grid cursor exactly where the user left it.
type assembledPart struct {
	Item   catalog.Item
	Picker colorPicker
}

// composePartsBody assembles a compose category's final body:
// "setopt PROMPT_SUBST" (without it, zsh never re-evaluates a
// $(command) embedded in PROMPT — parts like git-branch.zsh's
// PROMPT+='$(git_prompt_info)' would show up as that literal text
// instead of the actual branch name) plus a leading "PROMPT=\"\"" so
// the sequence starts from a clean slate regardless of whatever else
// may already be in the target file, then each part's raw body in
// order with "{{color}}" substituted for that part's chosen hex. Parts
// build up PROMPT incrementally (PROMPT+=...), the same idiom
// oh-my-zsh's own multi-line themes use.
func composePartsBody(cacheDir string, parts []assembledPart) (string, error) {
	var b strings.Builder
	b.WriteString("setopt PROMPT_SUBST\n")
	b.WriteString("PROMPT=\"\"\n")
	for _, p := range parts {
		raw, err := fetchItemBody(cacheDir, p.Item)
		if err != nil {
			return "", err
		}
		b.WriteString(strings.ReplaceAll(raw, "{{color}}", p.Picker.hex()))
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

func (m model) updateParts(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	parts := m.currentItemList()
	assembled := m.assembled[m.currentCategoryID]

	switch k.String() {
	case "left", "h", "esc":
		m.currentCategoryID = ""
		m.screen = screenCategories
		return m, nil

	case "tab":
		if len(assembled) > 0 {
			m.partsFocus = 1 - m.partsFocus
		}
		return m, nil

	case "up", "k":
		if m.partsFocus == 0 {
			if m.partsCursor > 0 {
				m.partsCursor--
			}
		} else if m.assembledCursor > 0 {
			m.assembledCursor--
		}
		return m, nil

	case "down", "j":
		if m.partsFocus == 0 {
			if m.partsCursor < len(parts)-1 {
				m.partsCursor++
			}
		} else if m.assembledCursor < len(assembled)-1 {
			m.assembledCursor++
		}
		return m, nil

	case " ", "enter":
		if m.partsFocus == 0 {
			if m.partsCursor < len(parts) {
				list := append(append([]assembledPart{}, assembled...), assembledPart{Item: parts[m.partsCursor]})
				m.assembled[m.currentCategoryID] = list
				m.editingIdx = len(list) - 1
				m.editingIsNewPart = true
				m.picker = newColorPicker()
				m.screen = screenPartColor
			}
		} else if m.assembledCursor < len(assembled) {
			m.editingIdx = m.assembledCursor
			m.editingIsNewPart = false
			m.picker = assembled[m.assembledCursor].Picker
			m.screen = screenPartColor
		}
		return m, nil

	case "K":
		if m.partsFocus == 1 && m.assembledCursor > 0 {
			list := append([]assembledPart{}, assembled...)
			list[m.assembledCursor-1], list[m.assembledCursor] = list[m.assembledCursor], list[m.assembledCursor-1]
			m.assembled[m.currentCategoryID] = list
			m.assembledCursor--
		}
		return m, nil

	case "J":
		if m.partsFocus == 1 && m.assembledCursor < len(assembled)-1 {
			list := append([]assembledPart{}, assembled...)
			list[m.assembledCursor+1], list[m.assembledCursor] = list[m.assembledCursor], list[m.assembledCursor+1]
			m.assembled[m.currentCategoryID] = list
			m.assembledCursor++
		}
		return m, nil

	case "x", "d":
		if m.partsFocus == 1 && m.assembledCursor < len(assembled) {
			list := append([]assembledPart{}, assembled[:m.assembledCursor]...)
			list = append(list, assembled[m.assembledCursor+1:]...)
			m.assembled[m.currentCategoryID] = list
			if m.assembledCursor >= len(list) && m.assembledCursor > 0 {
				m.assembledCursor--
			}
			if len(list) == 0 {
				m.partsFocus = 0
			}
		}
		return m, nil

	case "a":
		m.buildPlan()
		m.screen = screenConfirm
		return m, nil
	}
	return m, nil
}

func (m model) updatePartColor(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "left", "h":
		m.picker.move(-1, 0)
	case "right", "l":
		m.picker.move(1, 0)
	case "up", "k":
		m.picker.move(0, -1)
	case "down", "j":
		m.picker.move(0, 1)
	case "enter":
		list := m.assembled[m.currentCategoryID]
		if m.editingIdx < len(list) {
			list[m.editingIdx].Picker = m.picker
		}
		m.screen = screenParts
	case "esc":
		if m.editingIsNewPart {
			list := m.assembled[m.currentCategoryID]
			if m.editingIdx < len(list) {
				list = append(append([]assembledPart{}, list[:m.editingIdx]...), list[m.editingIdx+1:]...)
				m.assembled[m.currentCategoryID] = list
			}
		}
		m.screen = screenParts
	}
	return m, nil
}
