package apply

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyCreatesSectionInNewFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "zshrc")
	backupDir := filepath.Join(dir, "backup")

	if err := Apply(backupDir, target, "#", "shell", "export FOO=bar"); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	want := "# >>> carty: shell >>>\nexport FOO=bar\n# <<< carty: shell <<<\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestApplyPreservesExistingContentAndAppends(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "zshrc")
	backupDir := filepath.Join(dir, "backup")

	os.WriteFile(target, []byte("export EDITOR=vim\n"), 0644)

	if err := Apply(backupDir, target, "#", "shell", "export FOO=bar"); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(target)
	got := string(data)
	if got[:len("export EDITOR=vim\n")] != "export EDITOR=vim\n" {
		t.Fatalf("original content not preserved: %q", got)
	}
}

func TestApplyReplacesOnlyItsOwnSection(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "zshrc")
	backupDir := filepath.Join(dir, "backup")

	if err := Apply(backupDir, target, "#", "shell", "theme A"); err != nil {
		t.Fatal(err)
	}
	if err := Apply(backupDir, target, "#", "plugin", "plugin A"); err != nil {
		t.Fatal(err)
	}
	if err := Apply(backupDir, target, "#", "shell", "theme B"); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(target)
	got := string(data)

	if !strings.Contains(got, "theme B") {
		t.Fatalf("expected reapplied section content, got:\n%s", got)
	}
	if strings.Contains(got, "theme A") {
		t.Fatalf("stale section content should have been replaced, got:\n%s", got)
	}
	if !strings.Contains(got, "plugin A") {
		t.Fatalf("unrelated section should be untouched, got:\n%s", got)
	}
}

func TestApplyDetectsCorruptMarker(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "zshrc")
	backupDir := filepath.Join(dir, "backup")

	if err := Apply(backupDir, target, "#", "shell", "theme A"); err != nil {
		t.Fatal(err)
	}

	// Simulate the user hand-editing the file and deleting the end marker.
	data, _ := os.ReadFile(target)
	broken := strings.Replace(string(data), "# <<< carty: shell <<<\n", "", 1)
	os.WriteFile(target, []byte(broken), 0644)

	err := Apply(backupDir, target, "#", "shell", "theme B")
	if !errors.Is(err, ErrCorruptMarker) {
		t.Fatalf("expected ErrCorruptMarker, got %v", err)
	}
}

func TestRemoveDeletesSection(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "zshrc")
	backupDir := filepath.Join(dir, "backup")

	os.WriteFile(target, []byte("export EDITOR=vim\n"), 0644)
	Apply(backupDir, target, "#", "shell", "theme A")
	Apply(backupDir, target, "#", "plugin", "plugin A")

	if err := Remove(target, "#", "plugin"); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(target)
	got := string(data)
	if strings.Contains(got, "plugin A") {
		t.Fatalf("removed section still present:\n%s", got)
	}
	if !strings.Contains(got, "theme A") {
		t.Fatalf("unrelated section should survive removal:\n%s", got)
	}
	if !strings.Contains(got, "export EDITOR=vim") {
		t.Fatalf("original content should survive removal:\n%s", got)
	}
}

func TestRemoveIsNoopWhenSectionAbsent(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "zshrc")
	os.WriteFile(target, []byte("export EDITOR=vim\n"), 0644)

	before, _ := os.ReadFile(target)
	if err := Remove(target, "#", "shell"); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(target)
	if string(before) != string(after) {
		t.Fatalf("content changed on no-op remove: before=%q after=%q", before, after)
	}
}

func TestRestoreFromBackupUndoesAllChanges(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "zshrc")
	backupDir := filepath.Join(dir, "backup")
	original := "export EDITOR=vim\n"

	os.WriteFile(target, []byte(original), 0644)
	Apply(backupDir, target, "#", "shell", "theme A")

	if err := RestoreFromBackup(backupDir, target); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(target)
	if string(data) != original {
		t.Fatalf("got %q, want original %q", data, original)
	}
}
