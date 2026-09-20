package state

import (
	"path/filepath"
	"testing"
)

func TestLoadOrEmptyReturnsEmptyStateWhenMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	s, err := LoadOrEmpty(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(s) != 0 {
		t.Fatalf("expected empty state, got %+v", s)
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	want := State{
		"shell": Entry{ItemPath: "shell/zsh.sh", Item: "zsh", Version: "1.0.0", TargetFile: "/home/u/.zshrc"},
	}
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got["shell"] != want["shell"] {
		t.Fatalf("got %+v, want %+v", got["shell"], want["shell"])
	}
}
