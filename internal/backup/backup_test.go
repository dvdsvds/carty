package backup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureBackupCapturesOriginalOnlyOnce(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "zshrc")
	backupDir := filepath.Join(dir, "backup")

	os.WriteFile(target, []byte("original\n"), 0644)

	if err := EnsureBackup(backupDir, target); err != nil {
		t.Fatal(err)
	}
	if !Has(backupDir, target) {
		t.Fatal("expected backup to exist after first EnsureBackup")
	}

	// carty now modifies the file...
	os.WriteFile(target, []byte("modified by carty\n"), 0644)
	// ...and touches it again. The backup must NOT be overwritten with the
	// modified content — it should still reflect "original".
	if err := EnsureBackup(backupDir, target); err != nil {
		t.Fatal(err)
	}

	if err := Restore(backupDir, target); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(target)
	if string(data) != "original\n" {
		t.Fatalf("got %q, want the true original %q", data, "original\n")
	}
}

func TestEnsureBackupSkipsNonexistentFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "zshrc")
	backupDir := filepath.Join(dir, "backup")

	if err := EnsureBackup(backupDir, target); err != nil {
		t.Fatal(err)
	}
	if Has(backupDir, target) {
		t.Fatal("no backup should be recorded for a file that never existed")
	}
}

func TestRestoreRemovesFileCartyCreated(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "zshrc")
	backupDir := filepath.Join(dir, "backup")

	// carty creates a brand new file (no EnsureBackup call recorded a
	// backup because the file didn't exist beforehand).
	os.WriteFile(target, []byte("carty content\n"), 0644)

	if err := Restore(backupDir, target); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("expected target to be removed, stat err = %v", err)
	}
}

func TestRestoreOnAlreadyMissingFileIsNoop(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "zshrc")
	backupDir := filepath.Join(dir, "backup")

	if err := Restore(backupDir, target); err != nil {
		t.Fatal(err)
	}
}
