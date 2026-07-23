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
	cfg := config.FromEnv()
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
