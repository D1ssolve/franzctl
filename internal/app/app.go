package app

import (
	"fmt"
	"io"
)

const usage = `franzctl — a keyboard-first Apache Kafka workbench

Usage:
  franzctl                         launch the terminal UI
  franzctl topic create [flags]    create a topic
  franzctl consume [flags]         stream records
  franzctl produce [flags]         publish records
  franzctl connection <command>    manage saved broker connections
  franzctl codecs                  list codec stages
  franzctl version                 print build information

Run "franzctl <command> --help" for command-specific flags.
`

type App struct {
	version string
	commit  string
	date    string
}

func New(version, commit, date string) *App {
	return &App{version: version, commit: commit, date: date}
}

func (a *App) Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}

	switch args[0] {
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return 0
	case "version", "-v", "--version":
		fmt.Fprintf(stdout, "franzctl %s (commit=%s, built=%s)\n", a.version, a.commit, a.date)
		return 0
	case "codecs":
		return runCodecs(args[1:], stdout, stderr)
	case "connection", "connections":
		return runConnection(args[1:], stdin, stdout, stderr)
	case "topic":
		if len(args) < 2 || args[1] != "create" {
			fmt.Fprintln(stderr, `usage: franzctl topic create [flags]`)
			return 2
		}
		return runTopicCreate(args[2:], stdout, stderr)
	case "consume":
		return runConsume(args[1:], stdout, stderr)
	case "produce":
		return runProduce(args[1:], stdin, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n%s", args[0], usage)
		return 2
	}
}
