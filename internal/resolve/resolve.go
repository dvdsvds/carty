// Package resolve implements the cart-resolution rules from the README:
// auto-adding depends_on items and detecting alias/function collisions
// between items that would land in the same target file.
package resolve

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dvdsvds/carty/internal/catalog"
)

// ResolvedItem is a cart entry after dependency resolution.
type ResolvedItem struct {
	Item catalog.Item
	// Auto is true when the item was not chosen by the user directly but
	// pulled in because something else in the cart depends on it.
	Auto bool
}

// ErrCyclicDependency is returned when depends_on chains form a cycle.
// The catalog's build step is meant to reject this before publishing, but
// carty checks defensively at resolve time too.
type ErrCyclicDependency struct {
	Chain []string
}

func (e *ErrCyclicDependency) Error() string {
	return fmt.Sprintf("cyclic depends_on: %s", strings.Join(e.Chain, " -> "))
}

// Resolve expands selected items with their transitive depends_on, in
// stable order (user selections first, in selection order; then
// auto-added dependencies in the order they were discovered).
func Resolve(selected []catalog.Item, all []catalog.Item) ([]ResolvedItem, error) {
	byPath := make(map[string]catalog.Item, len(all))
	for _, it := range all {
		byPath[it.FilePath] = it
	}

	var out []ResolvedItem
	included := make(map[string]bool)

	var visit func(path string, chain []string, auto bool) error
	visit = func(path string, chain []string, auto bool) error {
		for _, c := range chain {
			if c == path {
				return &ErrCyclicDependency{Chain: append(append([]string{}, chain...), path)}
			}
		}
		if included[path] {
			return nil
		}
		item, ok := byPath[path]
		if !ok {
			return fmt.Errorf("resolve: unknown item %q", path)
		}

		included[path] = true
		out = append(out, ResolvedItem{Item: item, Auto: auto})

		for _, dep := range item.DependsOn {
			if err := visit(dep, append(chain, path), true); err != nil {
				return err
			}
		}
		return nil
	}

	for _, it := range selected {
		if err := visit(it.FilePath, nil, false); err != nil {
			return nil, err
		}
	}

	return out, nil
}

// Conflict describes two items that both define the same alias or
// function name and would be written into the same target file.
type Conflict struct {
	TargetFile string
	Name       string
	Kind       string // "alias" or "function"
	ItemA      string
	ItemB      string
}

// DetectConflicts checks every pair of resolved items that share a target
// file for overlapping provided alias/function names.
func DetectConflicts(items []ResolvedItem, targetFileFor func(categoryID string) string) []Conflict {
	type provided struct {
		itemName string
		kind     string
		name     string
	}

	byTarget := make(map[string][]provided)
	for _, ri := range items {
		target := targetFileFor(ri.Item.CategoryID)
		for _, a := range ri.Item.Provides.Aliases {
			byTarget[target] = append(byTarget[target], provided{ri.Item.Name, "alias", a})
		}
		for _, f := range ri.Item.Provides.Functions {
			byTarget[target] = append(byTarget[target], provided{ri.Item.Name, "function", f})
		}
	}

	var conflicts []Conflict
	for target, provs := range byTarget {
		for i := 0; i < len(provs); i++ {
			for j := i + 1; j < len(provs); j++ {
				a, b := provs[i], provs[j]
				if a.kind == b.kind && a.name == b.name && a.itemName != b.itemName {
					conflicts = append(conflicts, Conflict{
						TargetFile: target,
						Name:       a.name,
						Kind:       a.kind,
						ItemA:      a.itemName,
						ItemB:      b.itemName,
					})
				}
			}
		}
	}

	sort.Slice(conflicts, func(i, j int) bool {
		if conflicts[i].TargetFile != conflicts[j].TargetFile {
			return conflicts[i].TargetFile < conflicts[j].TargetFile
		}
		return conflicts[i].Name < conflicts[j].Name
	})

	return conflicts
}
