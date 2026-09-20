package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dvdsvds/carty/internal/appcmd"
	"github.com/dvdsvds/carty/internal/config"
	"github.com/dvdsvds/carty/internal/lock"
	"github.com/dvdsvds/carty/internal/selfupdate"
	"github.com/dvdsvds/carty/internal/tui"
)

var version = "dev"

func main() {
	if err := config.EnsureDir(); err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}

	if dir, err := config.Dir(); err == nil {
		selfupdate.CheckAndApply(dir, version)
	}

	args := os.Args[1:]
	if len(args) > 0 {
		if err := runSubcommand(args[0]); err != nil {
			fmt.Println("error:", err)
			os.Exit(1)
		}
		return
	}

	runTUI()
}

func runSubcommand(cmd string) error {
	switch cmd {
	case "--version", "-v", "version":
		fmt.Println("carty " + version)
		return nil
	case "update":
		return appcmd.Update()
	case "upgrade":
		return appcmd.Upgrade()
	case "uninstall":
		return appcmd.Uninstall(func() bool {
			return appcmd.ConfirmStdin("this cannot be undone. continue?")
		})
	default:
		return fmt.Errorf("unknown command %q (expected: update, upgrade, uninstall, --version)", cmd)
	}
}

func runTUI() {
	lockPath, err := config.LockPath()
	if err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}

	f, err := lock.Acquire(lockPath)
	if err != nil {
		fmt.Println("carty is already running.")
		os.Exit(1)
	}
	defer f.Close()
	defer lock.Release(lockPath)

	p := tea.NewProgram(tui.InitialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}
