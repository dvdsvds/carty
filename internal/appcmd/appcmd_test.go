package appcmd

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dvdsvds/carty/internal/apply"
	"github.com/dvdsvds/carty/internal/config"
	"github.com/dvdsvds/carty/internal/state"
)

// withFakeDataRepo starts a local server standing in for the data repo and
// points IndexURL/RawBaseURL at it for the duration of the test, restoring
// the real URLs afterward.
func withFakeDataRepo(t *testing.T, indexJSON string, files map[string]string) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/index.json", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(indexJSON))
	})
	for path, body := range files {
		body := body
		mux.HandleFunc("/files/"+path, func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(body))
		})
	}
	srv := httptest.NewServer(mux)

	origIndex, origBase := IndexURL, RawBaseURL
	IndexURL = srv.URL + "/index.json"
	RawBaseURL = srv.URL + "/"
	t.Cleanup(func() {
		srv.Close()
		IndexURL, RawBaseURL = origIndex, origBase
	})

	return srv
}

// withHome points config.Dir() (and everything derived from it) at a fresh
// temp directory for the duration of the test, and ensures ~/.carty exists
// the way main.go does before dispatching to any subcommand.
func withHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := config.EnsureDir(); err != nil {
		t.Fatal(err)
	}
	return home
}

const indexV1 = `{
  "categories": [
    {"id": "shell", "label": "Shell", "required": true, "priority": 1, "apply_method": "marker_section", "target_file": "%s", "comment_prefix": "#"}
  ],
  "items": [
    {"name": "Nord", "category_id": "shell", "file_path": "themes/zsh/nord.zsh", "version": "1.0.0", "depends_on": []}
  ]
}`

const indexV2 = `{
  "categories": [
    {"id": "shell", "label": "Shell", "required": true, "priority": 1, "apply_method": "marker_section", "target_file": "%s", "comment_prefix": "#"}
  ],
  "items": [
    {"name": "Nord", "category_id": "shell", "file_path": "themes/zsh/nord.zsh", "version": "2.0.0", "depends_on": []}
  ]
}`

func TestUpdateSavesIndexToCache(t *testing.T) {
	withHome(t)
	withFakeDataRepo(t, `{"categories":[],"items":[]}`, nil)

	if err := Update(); err != nil {
		t.Fatal(err)
	}

	cacheDir, _ := config.CachePath()
	if _, err := os.Stat(filepath.Join(cacheDir, "index.json")); err != nil {
		t.Fatalf("expected index.json to be cached: %v", err)
	}
}

func TestUpgradeAppliesNewerVersionAndUpdatesState(t *testing.T) {
	home := withHome(t)
	target := filepath.Join(home, ".zshrc")

	withFakeDataRepo(t, fmt.Sprintf(indexV1, target), map[string]string{
		"themes/zsh/nord.zsh": "---\nname: Nord\n---\ntheme v1 body",
	})

	// Simulate an already-installed v1.0.0.
	backupDir, _ := config.BackupDir()
	if err := apply.Apply(backupDir, target, "#", "shell", "theme v1 body"); err != nil {
		t.Fatal(err)
	}
	statePath, _ := config.StatePath()
	s := state.State{
		"shell": state.Entry{ItemPath: "themes/zsh/nord.zsh", Item: "Nord", Version: "1.0.0", TargetFile: target},
	}
	if err := state.Save(statePath, s); err != nil {
		t.Fatal(err)
	}

	// Point the fake repo at v2.0.0 and run upgrade.
	withFakeDataRepo(t, fmt.Sprintf(indexV2, target), map[string]string{
		"themes/zsh/nord.zsh": "---\nname: Nord\n---\ntheme v2 body",
	})

	if err := Upgrade(); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(target)
	if !strings.Contains(string(data), "theme v2 body") {
		t.Fatalf("expected upgraded body applied, got:\n%s", data)
	}

	got, err := state.Load(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if got["shell"].Version != "2.0.0" {
		t.Fatalf("expected state version updated to 2.0.0, got %+v", got["shell"])
	}
}

func TestUpgradeIsNoopWhenAlreadyLatest(t *testing.T) {
	home := withHome(t)
	target := filepath.Join(home, ".zshrc")

	withFakeDataRepo(t, fmt.Sprintf(indexV1, target), map[string]string{
		"themes/zsh/nord.zsh": "---\nname: Nord\n---\ntheme v1 body",
	})

	backupDir, _ := config.BackupDir()
	apply.Apply(backupDir, target, "#", "shell", "theme v1 body")
	statePath, _ := config.StatePath()
	state.Save(statePath, state.State{
		"shell": state.Entry{ItemPath: "themes/zsh/nord.zsh", Item: "Nord", Version: "1.0.0", TargetFile: target},
	})

	before, _ := os.ReadFile(target)

	if err := Upgrade(); err != nil {
		t.Fatal(err)
	}

	after, _ := os.ReadFile(target)
	if string(before) != string(after) {
		t.Fatalf("expected no changes when already at latest version:\nbefore=%s\nafter=%s", before, after)
	}
}

func TestUninstallRestoresBackupsAndRemovesCartyDir(t *testing.T) {
	home := withHome(t)
	target := filepath.Join(home, ".zshrc")
	original := "export EDITOR=vim\n"
	os.WriteFile(target, []byte(original), 0644)

	backupDir, _ := config.BackupDir()
	if err := apply.Apply(backupDir, target, "#", "shell", "theme v1 body"); err != nil {
		t.Fatal(err)
	}
	statePath, _ := config.StatePath()
	state.Save(statePath, state.State{
		"shell": state.Entry{ItemPath: "themes/zsh/nord.zsh", Item: "Nord", Version: "1.0.0", TargetFile: target},
	})

	if err := Uninstall(func() bool { return true }); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != original {
		t.Fatalf("got %q, want restored original %q", data, original)
	}

	dir, _ := config.Dir()
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("expected ~/.carty to be removed, stat err = %v", err)
	}
}

func TestUninstallCancelledLeavesEverythingUntouched(t *testing.T) {
	home := withHome(t)
	target := filepath.Join(home, ".zshrc")

	backupDir, _ := config.BackupDir()
	apply.Apply(backupDir, target, "#", "shell", "theme v1 body")
	statePath, _ := config.StatePath()
	state.Save(statePath, state.State{
		"shell": state.Entry{ItemPath: "themes/zsh/nord.zsh", Item: "Nord", Version: "1.0.0", TargetFile: target},
	})

	before, _ := os.ReadFile(target)

	if err := Uninstall(func() bool { return false }); err != nil {
		t.Fatal(err)
	}

	after, _ := os.ReadFile(target)
	if string(before) != string(after) {
		t.Fatal("cancelled uninstall must not modify the target file")
	}

	dir, _ := config.Dir()
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("expected ~/.carty to still exist after cancelling, stat err = %v", err)
	}
}
