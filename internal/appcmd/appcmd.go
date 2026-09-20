// Package appcmd implements the non-interactive carty subcommands:
// update, upgrade and uninstall.
package appcmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/dvdsvds/carty/internal/apply"
	"github.com/dvdsvds/carty/internal/backup"
	"github.com/dvdsvds/carty/internal/cache"
	"github.com/dvdsvds/carty/internal/catalog"
	"github.com/dvdsvds/carty/internal/config"
	"github.com/dvdsvds/carty/internal/fetch"
	"github.com/dvdsvds/carty/internal/state"
)

// IndexURL and RawBaseURL are vars (not consts) so tests can point them at
// a local httptest server instead of the real data repository.
var (
	IndexURL   = "https://raw.githubusercontent.com/dvdsvds/carty-data/main/index.json"
	RawBaseURL = "https://raw.githubusercontent.com/dvdsvds/carty-data/main/"
)

// Update refreshes the local cache of the data repository's index.json
// without touching any system files (mirrors `apt update`).
func Update() error {
	data, err := fetch.Get(IndexURL)
	if err != nil {
		return fmt.Errorf("update: fetch index: %w", err)
	}

	if _, err := catalog.ParseIndex(data); err != nil {
		return fmt.Errorf("update: parse index: %w", err)
	}

	cacheDir, err := config.CachePath()
	if err != nil {
		return err
	}
	if err := cache.SaveIndex(cacheDir, data); err != nil {
		return fmt.Errorf("update: save index: %w", err)
	}

	fmt.Println("index up to date.")
	return nil
}

// Upgrade applies newer versions of already-installed items (mirrors
// `apt upgrade`). It uses the cached index refreshed by Update, fetching
// one if no cache exists yet.
func Upgrade() error {
	cacheDir, err := config.CachePath()
	if err != nil {
		return err
	}

	data, err := cache.LoadIndex(cacheDir)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		if err := Update(); err != nil {
			return err
		}
		data, err = cache.LoadIndex(cacheDir)
		if err != nil {
			return err
		}
	}

	idx, err := catalog.ParseIndex(data)
	if err != nil {
		return fmt.Errorf("upgrade: parse index: %w", err)
	}
	byPath := make(map[string]catalog.Item, len(idx.Items))
	for _, it := range idx.Items {
		byPath[it.FilePath] = it
	}
	categoryByID := make(map[string]catalog.Category, len(idx.Categories))
	for _, c := range idx.Categories {
		categoryByID[c.ID] = c
	}

	statePath, err := config.StatePath()
	if err != nil {
		return err
	}
	s, err := state.LoadOrEmpty(statePath)
	if err != nil {
		return err
	}

	backupDir, err := config.BackupDir()
	if err != nil {
		return err
	}

	upgraded := 0
	for categoryID, entry := range s {
		item, ok := byPath[entry.ItemPath]
		if !ok {
			continue // no longer published; leave as-is, README says warn-only
		}
		if item.Version == entry.Version {
			continue
		}

		category, ok := categoryByID[categoryID]
		if !ok {
			continue
		}

		bodyData, err := fetch.Get(RawBaseURL + "files/" + item.FilePath)
		if err != nil {
			fmt.Printf("[FAIL] %s: %v\n", item.Name, err)
			continue
		}
		body := catalog.ExtractBody(bodyData)

		if err := apply.Apply(backupDir, entry.TargetFile, category.CommentPrefix, category.ID, body); err != nil {
			fmt.Printf("[FAIL] %s: %v\n", item.Name, err)
			continue
		}

		entry.Version = item.Version
		s[categoryID] = entry
		fmt.Printf("[OK] %s upgraded to %s\n", item.Name, item.Version)
		upgraded++
	}

	if err := state.Save(statePath, s); err != nil {
		return err
	}

	if upgraded == 0 {
		fmt.Println("everything already up to date.")
	}
	return nil
}

// Uninstall restores every target file to its pre-carty backup and removes
// ~/.carty entirely.
func Uninstall(confirm func() bool) error {
	statePath, err := config.StatePath()
	if err != nil {
		return err
	}
	backupDir, err := config.BackupDir()
	if err != nil {
		return err
	}
	dir, err := config.Dir()
	if err != nil {
		return err
	}

	s, err := state.LoadOrEmpty(statePath)
	if err != nil {
		return err
	}

	targets := make(map[string]bool)
	for _, entry := range s {
		targets[entry.TargetFile] = true
	}

	if len(targets) > 0 {
		fmt.Println("the following files will be restored to their pre-carty state:")
		for t := range targets {
			fmt.Println(" -", t)
		}
	}
	fmt.Printf("and %s will be removed entirely.\n", dir)

	if confirm != nil && !confirm() {
		fmt.Println("uninstall cancelled.")
		return nil
	}

	for t := range targets {
		if err := backup.Restore(backupDir, t); err != nil {
			fmt.Printf("[FAIL] restore %s: %v\n", t, err)
		}
	}

	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("uninstall: remove %s: %w", dir, err)
	}

	fmt.Println("carty uninstalled.")
	return nil
}

// ConfirmStdin asks a y/N question on stdin, used as Uninstall's confirm
// callback from the CLI entrypoint.
func ConfirmStdin(prompt string) bool {
	fmt.Print(prompt + " [y/N] ")
	var answer string
	fmt.Scanln(&answer)
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes"
}
