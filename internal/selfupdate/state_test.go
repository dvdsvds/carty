package selfupdate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestShouldCheckTrueWhenNeverChecked(t *testing.T) {
	dir := t.TempDir()
	if !ShouldCheck(dir) {
		t.Fatal("expected ShouldCheck to be true when no check has been recorded")
	}
}

func TestShouldCheckFalseRightAfterRecording(t *testing.T) {
	dir := t.TempDir()
	recordChecked(dir)
	if ShouldCheck(dir) {
		t.Fatal("expected ShouldCheck to be false immediately after recording a check")
	}
}

func TestPendingChangelogIsConsumedOnce(t *testing.T) {
	dir := t.TempDir()

	if pc, err := LoadPendingChangelog(dir); err != nil || pc != nil {
		t.Fatalf("expected no pending changelog, got %+v err=%v", pc, err)
	}

	if err := SavePendingChangelog(dir, "v0.2.0", "- added foo"); err != nil {
		t.Fatal(err)
	}

	pc, err := LoadPendingChangelog(dir)
	if err != nil {
		t.Fatal(err)
	}
	if pc == nil || pc.Version != "v0.2.0" || pc.Body != "- added foo" {
		t.Fatalf("unexpected pending changelog: %+v", pc)
	}

	pc2, err := LoadPendingChangelog(dir)
	if err != nil || pc2 != nil {
		t.Fatalf("expected changelog to be consumed after first load, got %+v err=%v", pc2, err)
	}

	if _, err := os.Stat(filepath.Join(dir, "pending_changelog.json")); !os.IsNotExist(err) {
		t.Fatalf("expected pending changelog file to be removed, stat err = %v", err)
	}
}
