package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/dvdsvds/carty/internal/catalog"
)

// Color palette. Keep it to a small, consistent set so every screen reads
// as one system rather than a stack of ad-hoc styles.
const (
	colorAccent   = "205" // pink/magenta — brand accent, selection, headers
	colorAccentFg = "234" // near-black, used as foreground on accent bg
	colorMuted    = "244" // secondary text, hints
	colorFaint    = "238" // borders, separators
	colorText     = "252" // primary text
	colorSuccess  = "78"
	colorFail     = "203"
	colorWarn     = "214"
)

var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorAccentFg)).
			Background(lipgloss.Color(colorAccent)).
			Padding(0, 2)

	breadcrumbStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorAccentFg)).
			Background(lipgloss.Color(colorAccent)).
			Padding(0, 2)

	footerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorText)).
			Background(lipgloss.Color(colorFaint)).
			Padding(0, 2)

	panelTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorAccent))

	panelBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color(colorFaint)).
				Padding(1, 2)

	rowStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorText)).
			Padding(0, 1)

	rowSelectedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(colorAccentFg)).
				Background(lipgloss.Color(colorAccent)).
				Padding(0, 1)

	mutedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted))
	badgeStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(colorWarn))
	errStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorFail))
	autoStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted)).Italic(true)
	okBadge     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorSuccess)).Render("OK")
	failBadge   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorFail)).Render("FAIL")
	checkOn     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorAccentFg)).Render("●")
	checkOff    = mutedStyle.Render("○")
	promptStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorAccent))
)

var screenTitles = map[screen]string{
	screenCategories: "카테고리",
	screenItems:      "항목 선택",
	screenParts:      "부품 조립",
	screenPartColor:  "색상 고르기",
	screenConfirm:    "적용 확인",
	screenCorrupt:    "마커 손상 감지",
	screenResults:    "적용 결과",
	screenChangelog:  "업데이트 알림",
}

// renderRow draws one full-width, selectable list row: a bar that fills
// the panel's interior width when focused, like a menu highlight, instead
// of a small cursor glyph. swatch, if non-empty, is a pre-rendered color
// preview appended after the highlighted bar (see renderSwatch) — it's
// rendered as its own segment because nesting differently-colored ANSI
// spans inside a single lipgloss.Style.Render call breaks the outer
// background after the first embedded reset code.
func renderRow(width int, focused bool, checked, label, swatch string) string {
	prefix := checkOff
	if checked != "" {
		prefix = checkOn
	}
	text := prefix + " " + label

	style := rowStyle
	if focused {
		style = rowSelectedStyle
	}
	row := style.Width(width).Render(text)
	if swatch == "" {
		return row
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, row, swatch)
}

// renderColorBlocks renders an item's raw preview_colors as solid blocks —
// the flat palette swatch, independent of the prompt mockup.
func renderColorBlocks(colors []string) string {
	var b strings.Builder
	for _, hex := range colors {
		b.WriteString(lipgloss.NewStyle().Background(lipgloss.Color(hex)).Render("   "))
	}
	return b.String()
}

// renderPromptPreview mocks up a powerline-style shell prompt segment using
// an item's preview_colors, so a theme reads as "what your terminal would
// actually look like" rather than three isolated dots. Data only ever
// supplies up to 3 colors (darkest ground, secondary ground, accent), so
// missing slots fall back to sensible neutral defaults.
func renderPromptPreview(colors []string) string {
	get := func(i int, fallback string) string {
		if i < len(colors) && colors[i] != "" {
			return colors[i]
		}
		return fallback
	}
	ground0 := get(0, "#2e3440")
	ground1 := get(1, "#3b4252")
	accent := get(2, "#88c0d0")

	chip := func(bg, fg, text string) string {
		return lipgloss.NewStyle().Background(lipgloss.Color(bg)).Foreground(lipgloss.Color(fg)).Padding(0, 1).Render(text)
	}

	host := chip(ground0, "#eceff4", "user@host")
	path := chip(ground1, accent, "~/project")
	arrow := lipgloss.NewStyle().Foreground(lipgloss.Color(accent)).Bold(true).Render(" ❯ ")

	return host + path + arrow
}

// renderItemCard draws one item as a bordered card: name as a title, a
// divider, then description/colors/prompt-preview/depends_on below it.
// Selection state is shown purely by border color/weight (no checkbox
// glyph): a faint border normally, accent-colored when the cursor is on
// it, and a heavier accent border when it's in the cart — selected wins
// over merely-focused so a selected card stays visibly marked even when
// the cursor moves elsewhere.
func renderItemCard(width int, focused, selected bool, it catalog.Item) string {
	border := lipgloss.RoundedBorder()
	borderColor := colorFaint
	switch {
	case selected:
		border = lipgloss.ThickBorder()
		borderColor = colorAccent
	case focused:
		borderColor = colorAccent
	}

	boxWidth := width - 2 // card border, 1 char each side
	if boxWidth < 3 {
		boxWidth = 3
	}
	textWidth := boxWidth - 2 // lipgloss.Width() includes the horizontal padding below

	style := lipgloss.NewStyle().
		Border(border).
		BorderForeground(lipgloss.Color(borderColor)).
		Padding(0, 1).
		Width(boxWidth)

	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorText)).Render(it.Name))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render(strings.Repeat("─", textWidth)))
	b.WriteString("\n")

	if it.Description != "" {
		b.WriteString(mutedStyle.Render(it.Description) + "\n")
	}
	if len(it.Previewcolors) > 0 {
		b.WriteString("\n" + renderColorBlocks(it.Previewcolors) + "\n")
		b.WriteString("\n" + renderPromptPreview(it.Previewcolors) + "\n")
	}
	if len(it.DependsOn) > 0 {
		b.WriteString("\n" + mutedStyle.Render("필요: "+strings.Join(it.DependsOn, ", ")))
	}

	return style.Render(strings.TrimRight(b.String(), "\n"))
}

func (m model) header() string {
	title := headerStyle.Render("carty")
	crumb := breadcrumbStyle.Render(screenTitles[m.screen])
	bar := lipgloss.JoinHorizontal(lipgloss.Top, title, crumb)

	pad := m.width - lipgloss.Width(bar)
	if pad > 0 {
		bar += headerStyle.Render(strings.Repeat(" ", pad))
	}
	return bar
}

// contentHeight is the number of lines left for the screen body once the
// (always 1-line) header and footer bars are accounted for.
func (m model) contentHeight() int {
	h := m.height - 2
	if h < 3 {
		h = 3
	}
	return h
}

func (m model) footer(hint string) string {
	text := hint
	if m.searching {
		text = "검색: " + m.searchInput + "█  (enter 확정 · esc 취소)"
	}
	style := footerStyle.Width(m.width)
	return style.Render(text)
}

// panel wraps content in a titled, rounded-border box sized to fit inside
// the given outer width/height.
func panel(title string, width, height int, content string) string {
	style := panelBorderStyle.Width(width - 4).Height(height - 3)
	body := content
	if title != "" {
		body = panelTitleStyle.Render(title) + "\n\n" + content
	}
	return style.Render(body)
}

func (m model) renderCartPanel(width, height int) string {
	var b strings.Builder
	if len(m.cart) == 0 {
		b.WriteString(mutedStyle.Render("담긴 항목이 없습니다."))
	} else {
		for _, c := range m.categories {
			if it, ok := m.cart[c.ID]; ok {
				fmt.Fprintf(&b, "%s\n%s\n\n", mutedStyle.Render(c.Label), it.Name)
			}
		}
	}
	return panel("장바구니", width, height, strings.TrimRight(b.String(), "\n"))
}

// minTermWidth/minTermHeight are the smallest terminal size carty's
// layout can render without any panel overflowing its budget (see
// panelBaseOverhead). Below this, instead of silently corrupting the
// layout, show a plain message asking for a bigger terminal — same as
// btop/lazygit/etc. do.
const minTermWidth = 60
const minTermHeight = 12

func (m model) View() string {
	if m.loadErr != "" {
		box := panel("오류", m.width, m.height, errStyle.Render(m.loadErr)+"\n\n"+mutedStyle.Render("q 로 종료"))
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
	}

	if m.width < minTermWidth || m.height < minTermHeight {
		msg := fmt.Sprintf("터미널이 너무 작습니다 (현재 %d×%d, 최소 %d×%d 필요)", m.width, m.height, minTermWidth, minTermHeight)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, errStyle.Render(msg))
	}

	header := m.header()
	footerHint := m.footerHint()
	footer := m.footer(footerHint)

	contentHeight := m.contentHeight()

	var body string
	switch m.screen {
	case screenChangelog:
		body = m.viewChangelog(m.width, contentHeight)
	case screenCategories:
		body = m.viewCategoriesAndCart(contentHeight)
	case screenItems:
		body = m.viewItemsAndCart(contentHeight)
	case screenParts:
		body = m.viewParts(contentHeight)
	case screenPartColor:
		body = m.viewPartColor(m.width, contentHeight)
	case screenConfirm:
		body = m.viewConfirm(m.width, contentHeight)
	case screenCorrupt:
		body = m.viewCorrupt(m.width, contentHeight)
	case screenResults:
		body = m.viewResults(m.width, contentHeight)
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

func (m model) footerHint() string {
	switch m.screen {
	case screenChangelog:
		return "아무 키나 눌러 계속"
	case screenCategories:
		hint := "→/enter 진입 · / 검색 · r 새로고침 · a 적용 확인 · q 종료"
		if m.notice != "" {
			hint = m.notice + "   " + hint
		}
		return hint
	case screenItems:
		return "←/h 뒤로 · space/enter 담기/빼기 · / 검색 · a 적용 확인 · q 종료"
	case screenParts:
		return "tab 목록/조립 전환 · enter 담기·색 편집 · J/K 순서 이동 · x 빼기 · ←/h 뒤로 · a 적용 확인 · q 종료"
	case screenPartColor:
		return "hjkl/방향키 색 이동 · enter 확정 · esc 취소"
	case screenConfirm:
		return "←/h 뒤로 · enter 적용 · q 종료"
	case screenCorrupt:
		return "y 복구 후 계속 · n 이 항목만 건너뛰기 · q 종료"
	case screenResults:
		return "enter/esc 로 카테고리 목록으로 · q 종료"
	}
	return "q 종료"
}

const panelGap = " "

// categoryRowWidth is the width viewCategoriesAndCart renders each
// category row at. Factored out so ensureCategoryVisible (model.go)
// measures rows at the exact same width they'll actually be drawn at.
func (m model) categoryRowWidth() int {
	cartWidth := 30
	if cartWidth > m.width/3 {
		cartWidth = m.width / 3
	}
	listWidth := m.width - cartWidth - len(panelGap)
	// listWidth - 8 = panel's exact usable text width (border 2 +
	// horizontal padding 4, see panel()); rows are flat text with no
	// border of their own, unlike item cards.
	return listWidth - 8 - scrollbarGutter
}

func categoryLabel(c catalog.Category) string {
	if c.Required {
		return c.Label + badgeStyle.Render(" (필수)")
	}
	return c.Label
}

// categoryRowHeights measures each category row's rendered line count,
// mirroring itemCardHeights.
func categoryRowHeights(width int, categories []catalog.Category) []int {
	heights := make([]int, len(categories))
	for i, c := range categories {
		heights[i] = lipgloss.Height(renderRow(width, false, "", categoryLabel(c), ""))
	}
	return heights
}

func (m model) viewCategoriesAndCart(height int) string {
	cartWidth := 30
	if cartWidth > m.width/3 {
		cartWidth = m.width / 3
	}
	listWidth := m.width - cartWidth - len(panelGap)

	innerWidth := m.categoryRowWidth()

	var body string
	if len(m.categories) == 0 {
		body = mutedStyle.Render("카테고리가 없습니다.")
	} else {
		avail := height - panelBaseOverhead
		if avail < 1 {
			avail = 1
		}

		start := m.catScroll
		if start >= len(m.categories) {
			start = len(m.categories) - 1
		}
		if start < 0 {
			start = 0
		}

		heights := categoryRowHeights(innerWidth, m.categories)
		used := 0
		end := start
		for i := start; i < len(m.categories); i++ {
			if used+heights[i] > avail && i > start {
				break
			}
			used += heights[i]
			end = i + 1
		}

		var rows strings.Builder
		for i := start; i < end; i++ {
			rows.WriteString(renderRow(innerWidth, m.catCursor == i, "", categoryLabel(m.categories[i]), ""))
			if i < end-1 {
				rows.WriteString("\n")
			}
		}

		rowsStr := rows.String()
		barHeight := lipgloss.Height(rowsStr)
		scrollbar := renderScrollbar(barHeight, len(m.categories), start, end)
		body = lipgloss.JoinHorizontal(lipgloss.Top, rowsStr, " ", scrollbar)
	}

	list := panel("", listWidth, height, body)
	cart := m.renderCartPanel(cartWidth, height)
	return lipgloss.JoinHorizontal(lipgloss.Top, list, panelGap, cart)
}

// panelBaseOverhead is how many lines of the height panel() is given get
// eaten before any of the body's own text: panel() calls
// panelBorderStyle.Height(height-3), and panelBorderStyle carries
// Padding(1,2) (2 more lines) — so border(2) + that "-3" + padding(2)
// nets out to height-5 being the max text height that avoids the box
// growing past `height` (lipgloss.Height() is a floor, not a cap: taller
// content simply overflows it). panelCardOverhead adds the title line
// and its trailing blank line (2), for panel() calls that pass a
// non-empty title (the item list and cart panel do; the category list,
// whose panel has no title, doesn't).
const panelBaseOverhead = 5
const panelCardOverhead = panelBaseOverhead + 2

// scrollbarGutter is the fixed width reserved for a scrollbar column
// (1 char bar + 1 char gap) — reserved unconditionally so row/card width
// never shifts depending on whether scrolling happens to be needed for
// the current list.
const scrollbarGutter = 2

// itemCardWidth is the width viewItemsAndCart renders each card at.
// Factored out so scroll math (ensureItemVisible, in model.go) measures
// cards at the exact same width they'll actually be drawn at.
func (m model) itemCardWidth() int {
	cartWidth := 30
	if cartWidth > m.width/3 {
		cartWidth = m.width / 3
	}
	listWidth := m.width - cartWidth - len(panelGap)
	// listWidth - 8 = panel's exact usable text width (panel border 2 +
	// its own horizontal padding 4); the card then adds its own border
	// (2) + padding (2) on top of that budget, computed in renderItemCard.
	return listWidth - 8 - scrollbarGutter
}

// renderScrollbar draws a height-tall vertical scrollbar column: a
// highlighted thumb spanning the visible fraction of the list
// ([start,end) out of total), on a faint track. Renders as a blank
// column when everything already fits (nothing to scroll).
func renderScrollbar(height, total, start, end int) string {
	track := lipgloss.NewStyle().Foreground(lipgloss.Color(colorFaint))
	thumb := lipgloss.NewStyle().Foreground(lipgloss.Color(colorAccent))

	if height < 1 {
		height = 1
	}
	visible := end - start
	if visible < 1 {
		visible = 1
	}
	if total <= visible {
		return strings.Repeat(" \n", height-1) + " "
	}

	thumbSize := height * visible / total
	if thumbSize < 1 {
		thumbSize = 1
	}
	if thumbSize > height {
		thumbSize = height
	}

	maxStart := total - visible
	thumbStart := 0
	if height > thumbSize && maxStart > 0 {
		thumbStart = (height - thumbSize) * start / maxStart
	}
	if thumbStart+thumbSize > height {
		thumbStart = height - thumbSize
	}

	var b strings.Builder
	for i := 0; i < height; i++ {
		if i > 0 {
			b.WriteString("\n")
		}
		if i >= thumbStart && i < thumbStart+thumbSize {
			b.WriteString(thumb.Render("┃"))
		} else {
			b.WriteString(track.Render("│"))
		}
	}
	return b.String()
}

// itemCardHeights measures each item's rendered line count. A card's
// height doesn't depend on focus/selection (those only change border
// glyphs/colors, not line count), so measuring with both false is safe
// to reuse for the real render too.
func itemCardHeights(width int, list []catalog.Item) []int {
	heights := make([]int, len(list))
	for i, it := range list {
		heights[i] = lipgloss.Height(renderItemCard(width, false, false, it))
	}
	return heights
}

func (m model) viewItemsAndCart(height int) string {
	cartWidth := 30
	if cartWidth > m.width/3 {
		cartWidth = m.width / 3
	}
	listWidth := m.width - cartWidth - len(panelGap)

	list := m.currentItemList()
	title := "항목"
	if m.searching {
		title = fmt.Sprintf("검색 결과: %q", m.searchInput)
	} else if c, ok := m.categoryByID[m.currentCategoryID]; ok {
		title = c.Label
	}

	innerWidth := m.itemCardWidth()

	var body string
	if len(list) == 0 {
		body = mutedStyle.Render("항목이 없습니다.")
	} else {
		avail := height - panelCardOverhead
		if avail < 1 {
			avail = 1
		}

		start := m.itemScroll
		if start >= len(list) {
			start = len(list) - 1
		}
		if start < 0 {
			start = 0
		}

		heights := itemCardHeights(innerWidth, list)
		used := 0
		end := start
		for i := start; i < len(list); i++ {
			addH := heights[i]
			if i > start {
				addH += 2
			}
			if used+addH > avail && i > start {
				break
			}
			used += addH
			end = i + 1
		}

		var rows strings.Builder
		for i := start; i < end; i++ {
			it := list[i]
			selected := false
			if sel, ok := m.cart[it.CategoryID]; ok && sel.FilePath == it.FilePath {
				selected = true
			}
			rows.WriteString(renderItemCard(innerWidth, m.itemCursor == i, selected, it))
			if i < end-1 {
				rows.WriteString("\n\n")
			}
		}

		rowsStr := rows.String()
		barHeight := lipgloss.Height(rowsStr)
		scrollbar := renderScrollbar(barHeight, len(list), start, end)
		body = lipgloss.JoinHorizontal(lipgloss.Top, rowsStr, " ", scrollbar)
	}

	listPanel := panel(title, listWidth, height, body)
	cart := m.renderCartPanel(cartWidth, height)
	return lipgloss.JoinHorizontal(lipgloss.Top, listPanel, panelGap, cart)
}

// viewParts renders the "compose" category screen: available parts on
// the left (browse and add, like the normal item list but multi-add
// instead of radio-select), and the assembled sequence on the right
// (reorder/recolor/remove). No scrolling — compose categories are
// expected to have a handful of parts, not dozens.
func (m model) viewParts(height int) string {
	rightWidth := 34
	if rightWidth > m.width/3 {
		rightWidth = m.width / 3
	}
	leftWidth := m.width - rightWidth - len(panelGap)
	innerWidth := leftWidth - 8

	parts := m.currentItemList()
	var left strings.Builder
	if len(parts) == 0 {
		left.WriteString(mutedStyle.Render("사용 가능한 부품이 없습니다."))
	} else {
		for i, it := range parts {
			focused := m.partsFocus == 0 && m.partsCursor == i
			left.WriteString(renderItemCard(innerWidth, focused, false, it))
			if i < len(parts)-1 {
				left.WriteString("\n\n")
			}
		}
	}
	leftPanel := panel("사용 가능한 부품", leftWidth, height, left.String())

	assembled := m.assembled[m.currentCategoryID]
	var right strings.Builder
	if len(assembled) == 0 {
		right.WriteString(mutedStyle.Render("아직 담긴 부품이 없습니다.\n왼쪽에서 space/enter로 담아보세요."))
	} else {
		rightInner := rightWidth - 6
		for i, ap := range assembled {
			dot := lipgloss.NewStyle().Foreground(lipgloss.Color(ap.Picker.hex())).Render("●")
			text := fmt.Sprintf("%s %d. %s", dot, i+1, ap.Item.Name)
			style := rowStyle
			if m.partsFocus == 1 && m.assembledCursor == i {
				style = rowSelectedStyle
			}
			right.WriteString(style.Width(rightInner).Render(text))
			right.WriteString("\n")
		}
	}
	rightPanel := panel("조립된 순서", rightWidth, height, strings.TrimRight(right.String(), "\n"))

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, panelGap, rightPanel)
}

// viewPartColor shows the color grid plus a live-colored preview of the
// part currently being edited, so moving the cursor visibly changes what
// that part will actually look like.
func (m model) viewPartColor(width, height int) string {
	list := m.assembled[m.currentCategoryID]
	name := ""
	if m.editingIdx < len(list) {
		name = list[m.editingIdx].Item.Name
	}

	preview := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(m.picker.hex())).Render(name)
	body := preview + "\n\n" + renderColorPicker(m.picker, "이 부품의 색")

	return panel(fmt.Sprintf("색상 고르기 — %s", name), width, height, body)
}

func (m model) viewConfirm(width, height int) string {
	var body strings.Builder
	if m.planErr != "" {
		body.WriteString(errStyle.Render(m.planErr))
	} else if len(m.conflicts) > 0 {
		body.WriteString(errStyle.Render("충돌이 발견되어 적용할 수 없습니다:") + "\n\n")
		for _, c := range m.conflicts {
			fmt.Fprintf(&body, "  %s: %s ↔ %s (%s \"%s\")\n", c.TargetFile, c.ItemA, c.ItemB, c.Kind, c.Name)
		}
	} else if len(m.plan) == 0 {
		body.WriteString(mutedStyle.Render("장바구니가 비어 있습니다."))
	} else {
		for _, pe := range m.plan {
			auto := ""
			if pe.Auto {
				auto = autoStyle.Render("  (자동으로 함께 추가됨)")
			}
			fmt.Fprintf(&body, "  %s\n    → %s%s\n\n", pe.Item.Name, mutedStyle.Render(pe.TargetFile), auto)
		}
		body.WriteString(promptStyle.Render("enter를 누르면 위 내용대로 실제 파일에 반영됩니다."))
	}
	return panel("적용 확인", width, height, strings.TrimRight(body.String(), "\n"))
}

func (m model) viewCorrupt(width, height int) string {
	var body strings.Builder
	if m.pendingCorrupt != nil {
		fmt.Fprintf(&body, "%s\n\n", mutedStyle.Render(m.pendingCorrupt.TargetFile))
	}
	body.WriteString(errStyle.Render("carty가 관리하는 구역의 마커가 손상되었습니다.") + "\n\n")
	body.WriteString("백업본으로 복구할까요? 복구하면 이 파일에 대한 사용자 수정 사항은 사라집니다.\n\n")
	body.WriteString(promptStyle.Render("y") + " 복구 후 계속하기   " + promptStyle.Render("n") + " 이 항목만 건너뛰기")
	return panel("마커 손상 감지", width, height, body.String())
}

func (m model) viewResults(width, height int) string {
	var body strings.Builder
	for _, r := range m.results {
		badge := okBadge
		detail := ""
		if r.Err != nil {
			badge = failBadge
			detail = mutedStyle.Render(": " + r.Err.Error())
		}
		fmt.Fprintf(&body, "[%s] %s%s\n", badge, r.Name, detail)
	}
	if len(m.results) == 0 {
		body.WriteString(mutedStyle.Render("적용된 항목이 없습니다."))
	}
	return panel("적용 결과", width, height, strings.TrimRight(body.String(), "\n"))
}

func (m model) viewChangelog(width, height int) string {
	title := "carty가 " + m.changelogVersion + "(으)로 자동 업데이트되었습니다"
	body := m.changelogBody + "\n\n" + promptStyle.Render("아무 키나 눌러 계속")
	return panel(title, width, height, body)
}
