package resolve

import (
	"errors"
	"testing"

	"github.com/dvdsvds/carty/internal/catalog"
)

func TestResolveAutoAddsDependency(t *testing.T) {
	all := []catalog.Item{
		{Name: "zsh", CategoryID: "shell", FilePath: "shell/zsh.sh"},
		{Name: "Nord", CategoryID: "zsh-theme", FilePath: "themes/zsh/nord.zsh", DependsOn: []string{"shell/zsh.sh"}},
	}
	nord := all[1]

	got, err := Resolve([]catalog.Item{nord}, all)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 resolved items, got %d: %+v", len(got), got)
	}

	if got[0].Item.FilePath != nord.FilePath || got[0].Auto {
		t.Fatalf("expected explicit selection first, got %+v", got[0])
	}
	if got[1].Item.FilePath != "shell/zsh.sh" || !got[1].Auto {
		t.Fatalf("expected auto-added dependency second, got %+v", got[1])
	}
}

func TestResolveDoesNotDuplicateAlreadySelectedDependency(t *testing.T) {
	all := []catalog.Item{
		{Name: "zsh", CategoryID: "shell", FilePath: "shell/zsh.sh"},
		{Name: "Nord", CategoryID: "zsh-theme", FilePath: "themes/zsh/nord.zsh", DependsOn: []string{"shell/zsh.sh"}},
	}

	got, err := Resolve(all, all)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected zsh to appear once even though it's both selected and depended on, got %+v", got)
	}
}

func TestResolveDetectsCycle(t *testing.T) {
	all := []catalog.Item{
		{Name: "A", FilePath: "a", DependsOn: []string{"b"}},
		{Name: "B", FilePath: "b", DependsOn: []string{"a"}},
	}

	_, err := Resolve([]catalog.Item{all[0]}, all)
	var cycleErr *ErrCyclicDependency
	if !errors.As(err, &cycleErr) {
		t.Fatalf("expected ErrCyclicDependency, got %v", err)
	}
}

func TestResolveErrorsOnUnknownDependency(t *testing.T) {
	all := []catalog.Item{
		{Name: "Nord", FilePath: "nord", DependsOn: []string{"does-not-exist"}},
	}

	_, err := Resolve([]catalog.Item{all[0]}, all)
	if err == nil {
		t.Fatal("expected an error for a depends_on referencing a nonexistent item")
	}
}

func TestDetectConflictsFindsOverlappingProvides(t *testing.T) {
	a := catalog.Item{Name: "Autosuggest", CategoryID: "zsh-plugin"}
	a.Provides.Functions = []string{"_zsh_autosuggest_start"}

	b := catalog.Item{Name: "Autosuggest2", CategoryID: "zsh-plugin2"}
	b.Provides.Functions = []string{"_zsh_autosuggest_start"}

	items := []ResolvedItem{{Item: a}, {Item: b}}

	conflicts := DetectConflicts(items, func(categoryID string) string { return "/home/u/.zshrc" })
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %+v", conflicts)
	}
	if conflicts[0].Name != "_zsh_autosuggest_start" || conflicts[0].Kind != "function" {
		t.Fatalf("unexpected conflict: %+v", conflicts[0])
	}
}

func TestDetectConflictsIgnoresDifferentTargetFiles(t *testing.T) {
	a := catalog.Item{Name: "A", CategoryID: "shell"}
	a.Provides.Aliases = []string{"ll"}
	b := catalog.Item{Name: "B", CategoryID: "nvim-plugin"}
	b.Provides.Aliases = []string{"ll"}

	items := []ResolvedItem{{Item: a}, {Item: b}}

	targetFileFor := func(categoryID string) string {
		if categoryID == "shell" {
			return "/home/u/.zshrc"
		}
		return "/home/u/.config/nvim/init.lua"
	}

	conflicts := DetectConflicts(items, targetFileFor)
	if len(conflicts) != 0 {
		t.Fatalf("expected no conflicts across different target files, got %+v", conflicts)
	}
}
