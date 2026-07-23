package main

import (
	"fmt"
	"os"
	"runtime/debug"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/D1ssolve/franzctl/internal/app"
	"github.com/D1ssolve/franzctl/internal/config"
	"github.com/D1ssolve/franzctl/internal/tui"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if len(os.Args) == 1 || (len(os.Args) == 2 && os.Args[1] == "tui") {
		if err := runTUI(); err != nil {
			fmt.Fprintln(os.Stderr, "franzctl:", err)
			os.Exit(1)
		}
		return
	}
	application := app.New(version, commit, date)
	os.Exit(application.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func runTUI() error {
	store, err := config.DefaultProfileStore()
	if err != nil {
		return err
	}
	profiles, err := store.List()
	if err != nil {
		return err
	}
	currentID := ""
	if current, currentErr := store.Current(); currentErr == nil {
		currentID = current.ID
	}
	selector := tui.NewConnectionSelector(profiles, currentID, config.FromEnv())
	selectionProgram := tea.NewProgram(selector, tea.WithAltScreen())
	selectedModel, err := selectionProgram.Run()
	if err != nil {
		return err
	}
	selected, ok := selectedModel.(tui.ConnectionSelector)
	if !ok {
		return fmt.Errorf("unexpected connection selector model %T", selectedModel)
	}
	selection, err := selected.Selection()
	if err != nil {
		return err
	}
	secrets := config.NewSecretStore(store)
	if selection.Created {
		if selection.Profile.CredentialsStored {
			if err := secrets.Set(selection.Profile.ID, selection.Credentials); err != nil {
				return err
			}
		}
		if err := store.Save(selection.Profile); err != nil {
			if selection.Profile.CredentialsStored {
				_ = secrets.Delete(selection.Profile.ID)
			}
			return err
		}
	}
	profile, err := store.Use(selection.Profile.ID)
	if err != nil {
		return err
	}
	cfg, err := config.ClientFromProfile(store, secrets, profile)
	if err != nil {
		return err
	}
	deps := app.BuildDependencies(cfg)
	model, err := tui.New(deps.Kafka, cfg.Brokers, resolveVersion())
	if err != nil {
		return err
	}
	program := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err = program.Run()
	return err
}

func resolveVersion() string {
	if version != "" && version != "dev" {
		return version
	}
	info, ok := debug.ReadBuildInfo()
	if ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}
