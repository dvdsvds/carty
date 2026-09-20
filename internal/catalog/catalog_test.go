package catalog

import (
	"os"
	"strings"
	"testing"
)

func TestResolveTargetFileUsesDefaultWhenEnvUnset(t *testing.T) {
	os.Unsetenv("CARTY_TEST_ZDOTDIR")
	c := Category{TargetFileEnv: "CARTY_TEST_ZDOTDIR", TargetFile: "~/.zshrc"}

	got := ResolveTargetFile(c, "/home/u")
	want := "/home/u/.zshrc"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestResolveTargetFilePrefersEnvWhenSet(t *testing.T) {
	os.Setenv("CARTY_TEST_ZDOTDIR", "/custom/zdotdir")
	defer os.Unsetenv("CARTY_TEST_ZDOTDIR")

	c := Category{TargetFileEnv: "CARTY_TEST_ZDOTDIR", TargetFile: "~/.zshrc"}

	got := ResolveTargetFile(c, "/home/u")
	want := "/custom/zdotdir"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestResolveTargetFileWithoutEnvVar(t *testing.T) {
	c := Category{TargetFile: "~/.config/nvim/init.lua"}
	got := ResolveTargetFile(c, "/home/u")
	want := "/home/u/.config/nvim/init.lua"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestParseIndex(t *testing.T) {
	data := []byte(`{
		"categories": [{"id": "shell", "label": "Shell", "required": true, "priority": 1, "target_file": "~/.zshrc"}],
		"items": [{"name": "Nord", "category_id": "shell", "file_path": "themes/zsh/nord.zsh", "version": "1.0.0"}]
	}`)

	idx, err := ParseIndex(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Categories) != 1 || idx.Categories[0].ID != "shell" {
		t.Fatalf("unexpected categories: %+v", idx.Categories)
	}
	if len(idx.Items) != 1 || idx.Items[0].FilePath != "themes/zsh/nord.zsh" {
		t.Fatalf("unexpected items: %+v", idx.Items)
	}
}

func TestExtractBody(t *testing.T) {
	data := []byte("---\nname: Nord\n---\nexport FOO=bar\n")
	got := ExtractBody(data)
	want := "export FOO=bar"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// A real oh-my-zsh theme (sunrise) sets PROMPTPREFIX="---" in its body —
// ExtractBody must not truncate at that literal "---".
func TestExtractBodyLiteralDashesInBody(t *testing.T) {
	data := []byte("---\nname: Sunrise\n---\nPROMPTPREFIX=\"---\"\nPROMPT='%{$PROMPTCOLOR%}$PROMPTPREFIX %~ %{$reset_color%}'\n")
	got := ExtractBody(data)
	if !strings.Contains(got, `PROMPTPREFIX="---"`) || !strings.Contains(got, "reset_color") {
		t.Fatalf("body truncated at literal ---, got %q", got)
	}
}
