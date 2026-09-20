package tui

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dvdsvds/carty/internal/apply"
	"github.com/dvdsvds/carty/internal/catalog"
	"github.com/dvdsvds/carty/internal/config"
	"github.com/dvdsvds/carty/internal/resolve"
	"github.com/dvdsvds/carty/internal/selfupdate"
	"github.com/dvdsvds/carty/internal/state"
)

const rawBaseURL = "https://raw.githubusercontent.com/dvdsvds/carty-data/main/"
const indexURL = rawBaseURL + "index.json"

type screen int

const (
	screenCategories screen = iota
	screenItems
	screenConfirm
	screenResults
	screenChangelog
	screenCorrupt
)

type ApplyResult struct {
	Name string
	Err  error
}

type planEntry struct {
	Item       catalog.Item
	Category   catalog.Category
	TargetFile string
	Auto       bool
}

type model struct {
	categories      []catalog.Category
	items           []catalog.Item
	itemsByCategory map[string][]catalog.Item
	categoryByID    map[string]catalog.Category

	home     string
	cacheDir string

	width  int
	height int

	screen     screen
	catCursor  int
	catScroll  int // index of the first category row currently visible
	itemCursor int
	itemScroll int // index of the first item card currently visible

	currentCategoryID string

	// cart holds at most one selected item per category id (radio select).
	cart map[string]catalog.Item

	searching     bool
	searchInput   string
	searchResults []catalog.Item

	plan      []planEntry
	conflicts []resolve.Conflict
	planErr   string

	// apply-in-progress state, used to pause and resume across a
	// screenCorrupt prompt.
	applyBackupDir    string
	applyStatePath    string
	applyState        state.State
	appliedCategories map[string]bool
	planIndex         int
	pendingCorrupt    *planEntry

	results []ApplyResult
	loadErr string
	notice  string

	changelogVersion string
	changelogBody    string
}

func InitialModel() model {
	home, _ := os.UserHomeDir()
	cacheDir, _ := config.CachePath()
	dir, _ := config.Dir()

	m := model{
		cart:         make(map[string]catalog.Item),
		categoryByID: make(map[string]catalog.Category),
		home:         home,
		cacheDir:     cacheDir,
		// sane defaults until the first tea.WindowSizeMsg arrives
		width:  80,
		height: 24,
	}

	data, fromCache, err := fetchIndexBytes(cacheDir)
	if err != nil {
		m.loadErr = "fetch error: " + err.Error()
		return m
	}
	if fromCache {
		m.notice = "오프라인: 캐시된 목록을 보여주고 있습니다."
	}

	if err := m.loadIndex(data); err != nil {
		m.loadErr = "parse error: " + err.Error()
		return m
	}

	if pc, err := selfupdate.LoadPendingChangelog(dir); err == nil && pc != nil {
		m.changelogVersion = pc.Version
		m.changelogBody = pc.Body
		m.screen = screenChangelog
	}

	return m
}

func (m *model) loadIndex(data []byte) error {
	idx, err := catalog.ParseIndex(data)
	if err != nil {
		return err
	}

	m.categories = idx.Categories
	sort.Slice(m.categories, func(i, j int) bool { return m.categories[i].Priority < m.categories[j].Priority })
	m.items = idx.Items

	m.itemsByCategory = make(map[string][]catalog.Item)
	for _, it := range idx.Items {
		m.itemsByCategory[it.CategoryID] = append(m.itemsByCategory[it.CategoryID], it)
	}
	m.categoryByID = make(map[string]catalog.Category)
	for _, c := range idx.Categories {
		m.categoryByID[c.ID] = c
	}

	// Drop cart entries for items that no longer exist in the refreshed index.
	for categoryID, it := range m.cart {
		found := false
		for _, candidate := range m.itemsByCategory[categoryID] {
			if candidate.FilePath == it.FilePath {
				found = true
				break
			}
		}
		if !found {
			delete(m.cart, categoryID)
		}
	}

	return nil
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) currentItemList() []catalog.Item {
	if m.searching {
		return m.searchResults
	}
	return m.itemsByCategory[m.currentCategoryID]
}

// ensureItemVisible clamps/advances itemScroll so the currently focused
// item's card is fully within the viewport, scrolling up immediately
// (cursor above the window) or down just far enough (cursor below it).
func (m *model) ensureItemVisible() {
	list := m.currentItemList()
	if len(list) == 0 {
		m.itemScroll = 0
		return
	}
	if m.itemCursor >= len(list) {
		m.itemCursor = len(list) - 1
	}
	if m.itemCursor < 0 {
		m.itemCursor = 0
	}
	if m.itemScroll > m.itemCursor {
		m.itemScroll = m.itemCursor
	}
	if m.itemScroll < 0 {
		m.itemScroll = 0
	}

	avail := m.contentHeight() - panelCardOverhead
	if avail < 1 {
		avail = 1
	}

	heights := itemCardHeights(m.itemCardWidth(), list)

	for {
		used := 0
		for i := m.itemScroll; i <= m.itemCursor; i++ {
			if i > m.itemScroll {
				used += 2
			}
			used += heights[i]
		}
		if used <= avail || m.itemScroll >= m.itemCursor {
			return
		}
		m.itemScroll++
	}
}

// ensureCategoryVisible is ensureItemVisible's counterpart for the
// category list.
func (m *model) ensureCategoryVisible() {
	if len(m.categories) == 0 {
		m.catScroll = 0
		return
	}
	if m.catCursor >= len(m.categories) {
		m.catCursor = len(m.categories) - 1
	}
	if m.catCursor < 0 {
		m.catCursor = 0
	}
	if m.catScroll > m.catCursor {
		m.catScroll = m.catCursor
	}
	if m.catScroll < 0 {
		m.catScroll = 0
	}

	avail := m.contentHeight() - panelBaseOverhead
	if avail < 1 {
		avail = 1
	}

	heights := categoryRowHeights(m.categoryRowWidth(), m.categories)

	for {
		used := 0
		for i := m.catScroll; i <= m.catCursor; i++ {
			used += heights[i]
		}
		if used <= avail || m.catScroll >= m.catCursor {
			return
		}
		m.catScroll++
	}
}

func (m *model) toggleSelect(item catalog.Item) {
	if existing, ok := m.cart[item.CategoryID]; ok && existing.FilePath == item.FilePath {
		delete(m.cart, item.CategoryID)
		return
	}
	m.cart[item.CategoryID] = item
}

func (m *model) runSearch() {
	q := strings.ToLower(strings.TrimSpace(m.searchInput))
	m.searchResults = nil
	if q == "" {
		return
	}
	for _, it := range m.items {
		if strings.Contains(strings.ToLower(it.Name), q) || strings.Contains(strings.ToLower(it.Description), q) {
			m.searchResults = append(m.searchResults, it)
		}
	}
}

func (m *model) buildPlan() {
	m.planErr = ""
	m.conflicts = nil
	m.plan = nil

	var selected []catalog.Item
	for _, it := range m.cart {
		selected = append(selected, it)
	}

	resolved, err := resolve.Resolve(selected, m.items)
	if err != nil {
		m.planErr = err.Error()
		return
	}

	for _, c := range m.categories {
		if !c.Required {
			continue
		}
		found := false
		for _, ri := range resolved {
			if ri.Item.CategoryID == c.ID {
				found = true
				break
			}
		}
		if !found {
			m.planErr = fmt.Sprintf("카테고리 %q는 최소 1개 선택이 필요합니다.", c.Label)
			return
		}
	}

	targetFileFor := func(categoryID string) string {
		return catalog.ResolveTargetFile(m.categoryByID[categoryID], m.home)
	}

	m.conflicts = resolve.DetectConflicts(resolved, targetFileFor)
	if len(m.conflicts) > 0 {
		return
	}

	for _, ri := range resolved {
		cat := m.categoryByID[ri.Item.CategoryID]
		m.plan = append(m.plan, planEntry{
			Item:       ri.Item,
			Category:   cat,
			TargetFile: catalog.ResolveTargetFile(cat, m.home),
			Auto:       ri.Auto,
		})
	}
	sort.SliceStable(m.plan, func(i, j int) bool { return m.plan[i].Category.Priority < m.plan[j].Category.Priority })
}

// startApply begins applying m.plan one entry at a time. It's split from
// advanceApply so a corrupt-marker prompt (screenCorrupt) can suspend the
// loop mid-way and resume it once the user answers.
func (m *model) startApply() {
	m.results = nil
	m.applyStatePath, _ = config.StatePath()
	m.applyBackupDir, _ = config.BackupDir()
	config.EnsureDir()

	s, err := state.LoadOrEmpty(m.applyStatePath)
	if err != nil {
		m.results = append(m.results, ApplyResult{Name: "state", Err: err})
		m.screen = screenResults
		return
	}

	m.applyState = s
	m.appliedCategories = make(map[string]bool)
	m.planIndex = 0
	m.pendingCorrupt = nil
	m.advanceApply()
}

func (m *model) advanceApply() {
	for m.planIndex < len(m.plan) {
		pe := m.plan[m.planIndex]
		m.appliedCategories[pe.Category.ID] = true

		body, err := fetchItemBody(m.cacheDir, pe.Item)
		if err != nil {
			m.results = append(m.results, ApplyResult{Name: pe.Item.Name, Err: err})
			m.planIndex++
			continue
		}

		err = apply.Apply(m.applyBackupDir, pe.TargetFile, pe.Category.CommentPrefix, pe.Category.ID, body)
		if err != nil {
			if errors.Is(err, apply.ErrCorruptMarker) {
				corrupt := pe
				m.pendingCorrupt = &corrupt
				m.screen = screenCorrupt
				return
			}
			m.results = append(m.results, ApplyResult{Name: pe.Item.Name, Err: err})
			m.planIndex++
			continue
		}

		m.applyState[pe.Category.ID] = state.Entry{
			ItemPath:   pe.Item.FilePath,
			Item:       pe.Item.Name,
			Version:    pe.Item.Version,
			TargetFile: pe.TargetFile,
		}
		m.results = append(m.results, ApplyResult{Name: pe.Item.Name, Err: nil})
		m.planIndex++
	}

	m.finishApply()
}

// finishApply removes sections for categories that were applied
// previously but aren't part of this cart, then persists state.
func (m *model) finishApply() {
	for categoryID, entry := range m.applyState {
		if m.appliedCategories[categoryID] {
			continue
		}
		cat, ok := m.categoryByID[categoryID]
		if !ok {
			continue
		}
		if err := apply.Remove(entry.TargetFile, cat.CommentPrefix, categoryID); err != nil {
			m.results = append(m.results, ApplyResult{Name: entry.Item + " (제거)", Err: err})
			continue
		}
		delete(m.applyState, categoryID)
	}

	if err := state.Save(m.applyStatePath, m.applyState); err != nil {
		m.results = append(m.results, ApplyResult{Name: "state", Err: err})
	}

	m.screen = screenResults
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if sizeMsg, ok := msg.(tea.WindowSizeMsg); ok {
		m.width, m.height = sizeMsg.Width, sizeMsg.Height
		return m, nil
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	if m.searching {
		return m.updateSearch(keyMsg)
	}

	switch keyMsg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "/":
		if m.screen == screenCategories || m.screen == screenItems {
			m.searching = true
			m.searchInput = ""
			m.searchResults = nil
			return m, nil
		}
	}

	switch m.screen {
	case screenChangelog:
		return m.updateChangelog(keyMsg)
	case screenCategories:
		return m.updateCategories(keyMsg)
	case screenItems:
		return m.updateItems(keyMsg)
	case screenConfirm:
		return m.updateConfirm(keyMsg)
	case screenCorrupt:
		return m.updateCorrupt(keyMsg)
	case screenResults:
		return m.updateResults(keyMsg)
	}
	return m, nil
}

func (m model) updateChangelog(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.screen = screenCategories
	return m, nil
}

func (m model) updateCorrupt(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.pendingCorrupt == nil {
		m.screen = screenResults
		return m, nil
	}

	switch k.String() {
	case "y":
		if err := apply.RestoreFromBackup(m.applyBackupDir, m.pendingCorrupt.TargetFile); err != nil {
			m.results = append(m.results, ApplyResult{Name: m.pendingCorrupt.Item.Name + " (백업 복구 실패)", Err: err})
			m.planIndex++
		}
		// else: retry the same index now that the corruption is gone.
		m.pendingCorrupt = nil
		m.advanceApply()
	case "n":
		m.results = append(m.results, ApplyResult{Name: m.pendingCorrupt.Item.Name, Err: apply.ErrCorruptMarker})
		m.planIndex++
		m.pendingCorrupt = nil
		m.advanceApply()
	}
	return m, nil
}

func (m model) updateSearch(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "esc":
		m.searching = false
		return m, nil
	case "enter":
		m.runSearch()
		m.screen = screenItems
		m.itemCursor = 0
		m.itemScroll = 0
		return m, nil
	case "backspace":
		if len(m.searchInput) > 0 {
			m.searchInput = m.searchInput[:len(m.searchInput)-1]
		}
		return m, nil
	default:
		if len(k.String()) == 1 {
			m.searchInput += k.String()
		}
		return m, nil
	}
}

func (m model) updateCategories(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "up", "k":
		if m.catCursor > 0 {
			m.catCursor--
			m.ensureCategoryVisible()
		}
	case "down", "j":
		if m.catCursor < len(m.categories)-1 {
			m.catCursor++
			m.ensureCategoryVisible()
		}
	case "right", "l", "enter":
		if len(m.categories) > 0 {
			m.currentCategoryID = m.categories[m.catCursor].ID
			m.itemCursor = 0
			m.itemScroll = 0
			m.screen = screenItems
		}
	case "a":
		m.buildPlan()
		m.screen = screenConfirm
	case "r":
		data, fromCache, err := fetchIndexBytes(m.cacheDir)
		if err != nil {
			m.notice = "새로고침 실패: " + err.Error()
		} else if err := m.loadIndex(data); err != nil {
			m.notice = "새로고침 실패: " + err.Error()
		} else if fromCache {
			m.notice = "오프라인: 캐시된 목록입니다."
		} else {
			m.notice = "목록을 새로고침했습니다."
		}
		if m.catCursor >= len(m.categories) {
			m.catCursor = 0
		}
		m.ensureCategoryVisible()
	}
	return m, nil
}

func (m model) updateItems(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	list := m.currentItemList()
	switch k.String() {
	case "up", "k":
		if m.itemCursor > 0 {
			m.itemCursor--
			m.ensureItemVisible()
		}
	case "down", "j":
		if m.itemCursor < len(list)-1 {
			m.itemCursor++
			m.ensureItemVisible()
		}
	case "left", "h", "esc":
		m.searching = false
		m.currentCategoryID = ""
		m.screen = screenCategories
	case " ", "enter", "right", "l":
		if m.itemCursor < len(list) {
			m.toggleSelect(list[m.itemCursor])
		}
	case "a":
		m.buildPlan()
		m.screen = screenConfirm
	}
	return m, nil
}

func (m model) updateConfirm(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "left", "h", "esc":
		m.screen = screenCategories
	case "enter":
		if m.planErr == "" && len(m.conflicts) == 0 {
			m.startApply()
		}
	}
	return m, nil
}

func (m model) updateResults(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "left", "h", "esc", "enter":
		m.screen = screenCategories
	}
	return m, nil
}
