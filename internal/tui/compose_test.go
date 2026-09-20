package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dvdsvds/carty/internal/cache"
	"github.com/dvdsvds/carty/internal/catalog"
)

func key(s string) tea.KeyMsg {
	switch s {
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case " ":
		return tea.KeyMsg{Type: tea.KeySpace}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func composeTestModel() model {
	parts := []catalog.Item{
		{Name: "사용자@호스트", CategoryID: "zsh-custom", FilePath: "zsh-custom/user-host.zsh"},
		{Name: "현재 경로", CategoryID: "zsh-custom", FilePath: "zsh-custom/path.zsh"},
	}
	return model{
		categories: []catalog.Category{
			{ID: "zsh-custom", Label: "Zsh 커스텀", ApplyMethod: "compose", TargetFile: "~/.zshrc", CommentPrefix: "#"},
		},
		itemsByCategory:   map[string][]catalog.Item{"zsh-custom": parts},
		categoryByID:      map[string]catalog.Category{"zsh-custom": {ID: "zsh-custom", Label: "Zsh 커스텀", ApplyMethod: "compose", TargetFile: "~/.zshrc", CommentPrefix: "#"}},
		cart:              map[string]catalog.Item{},
		assembled:         map[string][]assembledPart{},
		currentCategoryID: "zsh-custom",
		screen:            screenParts,
		width:             100,
		height:            30,
	}
}

func TestComposePartsBodySubstitutesColorAndPrependsCleanSlate(t *testing.T) {
	cacheDir := t.TempDir()

	if err := cache.SaveFile(cacheDir, "zsh-custom/a.zsh", []byte("---\nname: A\n---\nPROMPT+=\"%F{{{color}}}A%f\"")); err != nil {
		t.Fatal(err)
	}
	if err := cache.SaveFile(cacheDir, "zsh-custom/b.zsh", []byte("---\nname: B\n---\nPROMPT+=\"%F{{{color}}}B%f\"")); err != nil {
		t.Fatal(err)
	}

	parts := []assembledPart{
		{Item: catalog.Item{FilePath: "zsh-custom/a.zsh"}, Picker: colorPicker{hueIndex: 0, satIndex: 0}},
		{Item: catalog.Item{FilePath: "zsh-custom/b.zsh"}, Picker: colorPicker{hueIndex: 16, satIndex: 0}},
	}

	body, err := composePartsBody(cacheDir, parts)
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(body, "\n")
	if lines[0] != `PROMPT=""` {
		t.Fatalf("expected body to start with a clean PROMPT reset, got %q", lines[0])
	}
	if !strings.Contains(body, parts[0].Picker.hex()+"}A") {
		t.Fatalf("part A's color not substituted correctly, got:\n%s", body)
	}
	if !strings.Contains(body, parts[1].Picker.hex()+"}B") {
		t.Fatalf("part B's color not substituted correctly, got:\n%s", body)
	}
	if strings.Contains(body, "{{color}}") {
		t.Fatalf("a {{color}} placeholder was left unsubstituted:\n%s", body)
	}
}

func TestUpdatePartsAddOpensColorPickerForNewPart(t *testing.T) {
	m := composeTestModel()

	updated, _ := m.updateParts(key("enter")) // add the first available part
	m2 := updated.(model)

	if m2.screen != screenPartColor {
		t.Fatalf("expected adding a part to open the color picker, got screen %v", m2.screen)
	}
	if !m2.editingIsNewPart {
		t.Fatal("expected editingIsNewPart to be true for a freshly added part")
	}
	list := m2.assembled["zsh-custom"]
	if len(list) != 1 || list[0].Item.Name != "사용자@호스트" {
		t.Fatalf("expected the highlighted part to be added, got %+v", list)
	}
}

func TestEscOnNewPartRemovesIt(t *testing.T) {
	m := composeTestModel()
	updated, _ := m.updateParts(key("enter"))
	m2 := updated.(model)

	updated2, _ := m2.updatePartColor(key("esc"))
	m3 := updated2.(model)

	if m3.screen != screenParts {
		t.Fatalf("expected esc to return to screenParts, got %v", m3.screen)
	}
	if len(m3.assembled["zsh-custom"]) != 0 {
		t.Fatalf("expected esc on a brand-new part to remove it, got %+v", m3.assembled["zsh-custom"])
	}
}

func TestEscOnExistingPartKeepsItsColor(t *testing.T) {
	m := composeTestModel()
	updated, _ := m.updateParts(key("enter"))
	m2 := updated.(model)
	updated, _ = m2.updatePartColor(key("l")) // nudge the color
	m2 = updated.(model)
	updated, _ = m2.updatePartColor(key("enter")) // confirm it
	m2 = updated.(model)
	confirmedColor := m2.assembled["zsh-custom"][0].Picker

	// re-open the same part's color editor and back out without confirming
	m2.partsFocus = 1
	m2.assembledCursor = 0
	updated, _ = m2.updateParts(key("enter"))
	m3 := updated.(model)
	updated, _ = m3.updatePartColor(key("l")) // nudge again, but cancel
	m3 = updated.(model)
	updated, _ = m3.updatePartColor(key("esc"))
	m4 := updated.(model)

	if len(m4.assembled["zsh-custom"]) != 1 {
		t.Fatalf("esc on an existing part must not delete it, got %+v", m4.assembled["zsh-custom"])
	}
	if m4.assembled["zsh-custom"][0].Picker != confirmedColor {
		t.Fatalf("esc must discard the in-progress nudge and keep the previously confirmed color, got %+v want %+v",
			m4.assembled["zsh-custom"][0].Picker, confirmedColor)
	}
}

func TestReorderWithJK(t *testing.T) {
	m := composeTestModel()

	// add both available parts, confirming default colors each time
	m.partsCursor = 0
	updated, _ := m.updateParts(key("enter"))
	m = updated.(model)
	updated, _ = m.updatePartColor(key("enter"))
	m = updated.(model)

	m.partsCursor = 1
	updated, _ = m.updateParts(key("enter"))
	m = updated.(model)
	updated, _ = m.updatePartColor(key("enter"))
	m = updated.(model)

	list := m.assembled["zsh-custom"]
	if len(list) != 2 {
		t.Fatalf("expected 2 assembled parts, got %d", len(list))
	}
	first := list[0].Item.Name

	m.partsFocus = 1
	m.assembledCursor = 1
	updated, _ = m.updateParts(key("K")) // move second part up
	m = updated.(model)

	newList := m.assembled["zsh-custom"]
	if newList[0].Item.Name == first {
		t.Fatalf("expected K to move the second part before the first, order unchanged: %+v", newList)
	}
	if m.assembledCursor != 0 {
		t.Fatalf("expected cursor to follow the moved part to index 0, got %d", m.assembledCursor)
	}
}

func TestRemoveWithX(t *testing.T) {
	m := composeTestModel()
	updated, _ := m.updateParts(key("enter"))
	m = updated.(model)
	updated, _ = m.updatePartColor(key("enter"))
	m = updated.(model)

	if len(m.assembled["zsh-custom"]) != 1 {
		t.Fatalf("setup failed, expected 1 assembled part")
	}

	m.partsFocus = 1
	m.assembledCursor = 0
	updated, _ = m.updateParts(key("x"))
	m = updated.(model)

	if len(m.assembled["zsh-custom"]) != 0 {
		t.Fatalf("expected x to remove the part, got %+v", m.assembled["zsh-custom"])
	}
	if m.partsFocus != 0 {
		t.Fatalf("expected focus to fall back to the available-parts list once the sequence is empty, got focus=%d", m.partsFocus)
	}
}
