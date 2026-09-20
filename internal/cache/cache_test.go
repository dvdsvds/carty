package cache

import (
	"os"
	"testing"
)

func TestIndexRoundTrip(t *testing.T) {
	dir := t.TempDir()

	if _, err := LoadIndex(dir); err == nil {
		t.Fatal("expected an error loading an index that was never cached")
	}

	want := []byte(`{"categories":[],"items":[]}`)
	if err := SaveIndex(dir, want); err != nil {
		t.Fatal(err)
	}

	got, err := LoadIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFileRoundTripWithNestedPath(t *testing.T) {
	dir := t.TempDir()

	if err := SaveFile(dir, "themes/zsh/nord.zsh", []byte("body")); err != nil {
		t.Fatal(err)
	}

	got, err := LoadFile(dir, "themes/zsh/nord.zsh")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "body" {
		t.Fatalf("got %q, want %q", got, "body")
	}
}

func TestLoadFileMissingReturnsError(t *testing.T) {
	dir := t.TempDir()
	if _, err := LoadFile(dir, "nope.zsh"); !os.IsNotExist(err) {
		t.Fatalf("expected a not-exist error, got %v", err)
	}
}
