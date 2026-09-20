package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dvdsvds/carty/internal/apply"
	"github.com/dvdsvds/carty/internal/catalog"
	"github.com/dvdsvds/carty/internal/state"
)

// TestCorruptMarkerFlowPausesAndResumes exercises the interactive
// recovery path: advanceApply must stop at screenCorrupt instead of
// silently overwriting a hand-edited file, and answering "y" must restore
// the backup and successfully retry the same plan entry.
//
// It drives advanceApply/updateCorrupt directly (bypassing startApply,
// which always points at the real ~/.carty paths) so it can use a
// temporary backup dir and a cached item body instead of hitting the
// network.
func TestCorruptMarkerFlowPausesAndResumes(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "zshrc")
	backupDir := filepath.Join(dir, "backup")
	cacheDir := filepath.Join(dir, "cache")

	original := "export EDITOR=vim\n"
	os.WriteFile(target, []byte(original), 0644)

	category := catalog.Category{ID: "shell", CommentPrefix: "#", TargetFile: target}
	if err := apply.Apply(backupDir, target, category.CommentPrefix, category.ID, "theme A"); err != nil {
		t.Fatal(err)
	}

	// User hand-edits the file and deletes the end marker.
	data, _ := os.ReadFile(target)
	broken := strings.Replace(string(data), "# <<< carty: shell <<<\n", "", 1)
	os.WriteFile(target, []byte(broken), 0644)

	// Seed the cache so the retry's fetch doesn't need the network.
	os.MkdirAll(filepath.Join(cacheDir, "files", "carty-test-fixtures"), 0755)
	os.WriteFile(filepath.Join(cacheDir, "files", "carty-test-fixtures", "nonexistent-nord.zsh"), []byte("---\nname: Nord\n---\ntheme B"), 0644)

	m := model{
		cart:              map[string]catalog.Item{},
		categoryByID:      map[string]catalog.Category{"shell": category},
		home:              dir,
		cacheDir:          cacheDir,
		applyBackupDir:    backupDir,
		applyStatePath:    filepath.Join(dir, "state.json"),
		applyState:        state.State{},
		appliedCategories: map[string]bool{},
		plan: []planEntry{
			{Item: catalog.Item{Name: "Nord", CategoryID: "shell", FilePath: "carty-test-fixtures/nonexistent-nord.zsh"}, Category: category, TargetFile: target},
		},
	}

	m.advanceApply()

	if m.screen != screenCorrupt {
		t.Fatalf("expected advanceApply to pause at screenCorrupt, got %v (results=%+v)", m.screen, m.results)
	}
	if m.pendingCorrupt == nil {
		t.Fatal("expected pendingCorrupt to be set")
	}
	if len(m.results) != 0 {
		t.Fatalf("expected no results recorded yet, got %+v", m.results)
	}

	updated, _ := m.updateCorrupt(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	m2 := updated.(model)

	if m2.screen != screenResults {
		t.Fatalf("expected screenResults after restore+retry, got %v (results=%+v)", m2.screen, m2.results)
	}
	if len(m2.results) != 1 || m2.results[0].Err != nil {
		t.Fatalf("expected one successful result, got %+v", m2.results)
	}

	final, _ := os.ReadFile(target)
	if !strings.Contains(string(final), original) {
		t.Fatalf("expected original content preserved, got:\n%s", final)
	}
	if !strings.Contains(string(final), "theme B") {
		t.Fatalf("expected retried apply to have landed, got:\n%s", final)
	}
}

// TestCorruptMarkerFlowSkipOnNo confirms that answering "n" records a
// failure for that item and moves on rather than looping forever.
func TestCorruptMarkerFlowSkipOnNo(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "zshrc")
	backupDir := filepath.Join(dir, "backup")
	cacheDir := filepath.Join(dir, "cache")

	category := catalog.Category{ID: "shell", CommentPrefix: "#", TargetFile: target}
	apply.Apply(backupDir, target, category.CommentPrefix, category.ID, "theme A")

	data, _ := os.ReadFile(target)
	broken := strings.Replace(string(data), "# <<< carty: shell <<<\n", "", 1)
	os.WriteFile(target, []byte(broken), 0644)

	os.MkdirAll(filepath.Join(cacheDir, "files", "carty-test-fixtures"), 0755)
	os.WriteFile(filepath.Join(cacheDir, "files", "carty-test-fixtures", "nonexistent-nord.zsh"), []byte("---\nname: Nord\n---\ntheme B"), 0644)

	m := model{
		cart:              map[string]catalog.Item{},
		categoryByID:      map[string]catalog.Category{"shell": category},
		home:              dir,
		cacheDir:          cacheDir,
		applyBackupDir:    backupDir,
		applyStatePath:    filepath.Join(dir, "state.json"),
		applyState:        state.State{},
		appliedCategories: map[string]bool{},
		plan: []planEntry{
			{Item: catalog.Item{Name: "Nord", CategoryID: "shell", FilePath: "carty-test-fixtures/nonexistent-nord.zsh"}, Category: category, TargetFile: target},
		},
	}

	m.advanceApply()
	if m.screen != screenCorrupt {
		t.Fatalf("expected screenCorrupt, got %v", m.screen)
	}

	updated, _ := m.updateCorrupt(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	m2 := updated.(model)

	if m2.screen != screenResults {
		t.Fatalf("expected screenResults after skip, got %v", m2.screen)
	}
	if len(m2.results) != 1 || m2.results[0].Err == nil {
		t.Fatalf("expected one failed result recorded, got %+v", m2.results)
	}

	// The file should be left exactly as it was (still corrupt) — "n"
	// must not touch the filesystem.
	final, _ := os.ReadFile(target)
	if string(final) != broken {
		t.Fatalf("expected corrupt file to be left untouched, got:\n%s", final)
	}
}
