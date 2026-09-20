package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/dvdsvds/carty/internal/catalog"
)

// TestEnsureCategoryVisibleNeverOverflows is a regression test for a real
// bug: ensureCategoryVisible copied ensureItemVisible's "avail < 3 ->
// avail = 3" floor, which made sense for multi-line item cards but was
// wrong for single-line category rows — on a small terminal that floor
// silently claimed more room than actually existed, so the scroll offset
// never advanced and the panel rendered taller than its Height() budget
// (pushing the header/footer off screen instead of actually scrolling).
// A second, related bug this also catches: the panelBaseOverhead/
// panelCardOverhead constants were off by one against what panel()
// actually does (Height(height-3) plus Padding(1,2)), and renderCartPanel
// didn't respect the height budget at all.
func TestEnsureCategoryVisibleNeverOverflows(t *testing.T) {
	var categories []catalog.Category
	for i := 0; i < 10; i++ {
		categories = append(categories, catalog.Category{ID: fmt.Sprintf("c%d", i), Label: fmt.Sprintf("Category %d", i)})
	}

	m := model{
		categories: categories,
		cart:       map[string]catalog.Item{},
		width:      90,
		height:     14, // small but a real, supported terminal size (>= minTermHeight)
	}

	m.catCursor = len(categories) - 1
	m.ensureCategoryVisible()

	if m.catScroll == 0 {
		t.Fatalf("expected catScroll to advance so the last category is visible, stayed at 0")
	}

	out := m.View()
	if got := lipgloss.Height(out); got > m.height {
		t.Fatalf("rendered view is %d lines tall, exceeds the terminal's %d lines", got, m.height)
	}
	if !strings.Contains(out, "Category 9") {
		t.Fatalf("expected the focused (last) category to actually be visible, got:\n%s", out)
	}
}

// TestEnsureItemVisibleNeverOverflows is TestEnsureCategoryVisibleNeverOverflows's
// counterpart for the item card list.
func TestEnsureItemVisibleNeverOverflows(t *testing.T) {
	var items []catalog.Item
	for i := 0; i < 6; i++ {
		items = append(items, catalog.Item{Name: fmt.Sprintf("Item %d", i), CategoryID: "zsh-theme", FilePath: fmt.Sprintf("f%d", i)})
	}

	m := model{
		categories:        []catalog.Category{{ID: "zsh-theme", Label: "Zsh Theme"}},
		itemsByCategory:   map[string][]catalog.Item{"zsh-theme": items},
		categoryByID:      map[string]catalog.Category{"zsh-theme": {ID: "zsh-theme", Label: "Zsh Theme"}},
		cart:              map[string]catalog.Item{},
		currentCategoryID: "zsh-theme",
		screen:            screenItems,
		width:             90,
		height:            14,
	}

	m.itemCursor = len(items) - 1
	m.ensureItemVisible()

	if m.itemScroll == 0 {
		t.Fatalf("expected itemScroll to advance so the last item is visible, stayed at 0")
	}

	out := m.View()
	if got := lipgloss.Height(out); got > m.height {
		t.Fatalf("rendered view is %d lines tall, exceeds the terminal's %d lines", got, m.height)
	}
	if !strings.Contains(out, "Item 5") {
		t.Fatalf("expected the focused (last) item to actually be visible, got:\n%s", out)
	}
}

// TestViewShowsMinSizeMessageBelowThreshold checks the too-small-terminal
// guard fires instead of trying to render (and overflow) the real layout.
func TestViewShowsMinSizeMessageBelowThreshold(t *testing.T) {
	m := model{
		categories: []catalog.Category{{ID: "a", Label: "A"}},
		cart:       map[string]catalog.Item{},
		width:      minTermWidth,
		height:     minTermHeight - 1,
	}

	out := m.View()
	if !strings.Contains(out, "너무 작습니다") {
		t.Fatalf("expected the too-small-terminal message, got:\n%s", out)
	}
}
