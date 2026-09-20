// Package selfupdate implements carty's own binary auto-update, per the
// README: on its normal refresh cycle carty checks GitHub Releases for a
// newer version, overwrites its own binary if one exists, and shows a
// changelog summary the next time the TUI starts.
package selfupdate

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/dvdsvds/carty/internal/fetch"
)

const releasesURL = "https://api.github.com/repos/dvdsvds/carty/releases/latest"

// CheckInterval bounds how often ShouldCheck allows a new check, so carty
// doesn't hit the GitHub API on every single launch.
const CheckInterval = 24 * time.Hour

type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type Release struct {
	TagName string  `json:"tag_name"`
	Body    string  `json:"body"`
	Assets  []Asset `json:"assets"`
}

func LatestRelease() (*Release, error) {
	data, err := fetch.Get(releasesURL)
	if err != nil {
		return nil, err
	}

	var r Release
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

func NeedsUpdate(currentVersion string) (bool, *Release, error) {
	rel, err := LatestRelease()
	if err != nil {
		return false, nil, err
	}
	return rel.TagName != "" && rel.TagName != currentVersion, rel, nil
}

// SelectAsset finds the release asset matching this platform, named
// "carty_<goos>_<goarch>" by the release build (see stoke.toml packaging).
func SelectAsset(rel *Release, goos, goarch string) (Asset, error) {
	want := fmt.Sprintf("carty_%s_%s", goos, goarch)
	for _, a := range rel.Assets {
		if a.Name == want {
			return a, nil
		}
	}
	return Asset{}, fmt.Errorf("selfupdate: no release asset for %s", want)
}

func currentExecutablePath() (string, error) {
	return os.Executable()
}

// Apply downloads downloadURL and atomically replaces the currently
// running executable with it. The new binary only takes effect the next
// time carty is launched.
func Apply(downloadURL string) error {
	data, err := fetch.Get(downloadURL)
	if err != nil {
		return err
	}

	execPath, err := currentExecutablePath()
	if err != nil {
		return err
	}

	tmpPath := execPath + ".new"
	if err := os.WriteFile(tmpPath, data, 0755); err != nil {
		return err
	}

	return os.Rename(tmpPath, execPath)
}

// CheckAndApply runs one self-update cycle: if the release cadence allows
// a check, and a newer release exists with a matching asset, it downloads
// and installs it, then records a pending changelog for the next launch's
// TUI to display. All failures are non-fatal to the caller (network
// hiccups shouldn't block carty from running).
func CheckAndApply(stateDir, currentVersion string) {
	if currentVersion == "dev" {
		return
	}
	if !ShouldCheck(stateDir) {
		return
	}
	recordChecked(stateDir)

	needs, rel, err := NeedsUpdate(currentVersion)
	if err != nil || !needs {
		return
	}

	asset, err := SelectAsset(rel, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return
	}

	if err := Apply(asset.BrowserDownloadURL); err != nil {
		return
	}

	_ = SavePendingChangelog(stateDir, rel.TagName, rel.Body)
}
