package app

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/D1ssolve/franzctl/internal/config"
)

const connectionUsage = `Manage saved Kafka connections.

Usage:
  franzctl connection list [--json]
  franzctl connection show <name>
  franzctl connection use <name>
  franzctl connection add [flags]
  franzctl connection remove <name>

The active connection is used by the TUI and by CLI commands unless --broker
or FRANZCTL_BROKERS overrides it.
`

func runConnection(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, connectionUsage)
		return 2
	}
	store, err := config.DefaultProfileStore()
	if err != nil {
		fmt.Fprintln(stderr, "connections:", err)
		return 1
	}
	secrets := config.NewSecretStore(store)

	switch args[0] {
	case "help", "-h", "--help":
		fmt.Fprint(stdout, connectionUsage)
		return 0
	case "list", "ls":
		return listConnections(store, args[1:], stdout, stderr)
	case "show":
		return showConnection(store, args[1:], stdout, stderr)
	case "use":
		return useConnection(store, args[1:], stdout, stderr)
	case "add":
		return addConnection(store, secrets, args[1:], stdin, stdout, stderr)
	case "remove", "rm", "delete":
		return removeConnection(store, secrets, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown connection command %q\n\n%s", args[0], connectionUsage)
		return 2
	}
}

func listConnections(store *config.ProfileStore, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("connection list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "write JSON")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	profiles, err := store.List()
	if err != nil {
		fmt.Fprintln(stderr, "list connections:", err)
		return 1
	}
	if *asJSON {
		if err := json.NewEncoder(stdout).Encode(profiles); err != nil {
			fmt.Fprintln(stderr, "write output:", err)
			return 1
		}
		return 0
	}
	if len(profiles) == 0 {
		fmt.Fprintln(stdout, "No saved connections. Run: franzctl connection add --help")
		return 0
	}
	current, currentErr := store.Current()
	for _, profile := range profiles {
		marker := " "
		if currentErr == nil && profile.ID == current.ID {
			marker = "*"
		}
		auth := "none"
		if profile.SASLMechanism != "" {
			auth = profile.SASLMechanism
		}
		fmt.Fprintf(stdout, "%s %-20s %-36s tls=%t auth=%s\n",
			marker,
			profile.Name,
			strings.Join(profile.Brokers, ","),
			profile.TLSEnabled,
			auth,
		)
	}
	return 0
}

func showConnection(store *config.ProfileStore, args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: franzctl connection show <name>")
		return 2
	}
	profile, err := store.Resolve(args[0])
	if err != nil {
		fmt.Fprintln(stderr, "show connection:", err)
		return 1
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(profile); err != nil {
		fmt.Fprintln(stderr, "write output:", err)
		return 1
	}
	return 0
}

func useConnection(store *config.ProfileStore, args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: franzctl connection use <name>")
		return 2
	}
	profile, err := store.Use(args[0])
	if err != nil {
		fmt.Fprintln(stderr, "select connection:", err)
		return 1
	}
	fmt.Fprintf(stdout, "Using connection %q (%s).\n", profile.Name, strings.Join(profile.Brokers, ","))
	return 0
}

func addConnection(
	store *config.ProfileStore,
	secrets config.SecretStore,
	args []string,
	stdin io.Reader,
	stdout, stderr io.Writer,
) int {
	fs := flag.NewFlagSet("connection add", flag.ContinueOnError)
	fs.SetOutput(stderr)
	client := config.Client{
		ClientID:       "franzctl",
		RequestTimeout: 15 * time.Second,
	}
	var name, username string
	var passwordStdin bool
	fs.StringVar(&name, "name", "", "connection name (required)")
	fs.Var(&client.Brokers, "broker", "Kafka bootstrap broker; repeat or use a comma-separated list")
	fs.StringVar(&client.ClientID, "client-id", client.ClientID, "Kafka client ID")
	fs.DurationVar(&client.RequestTimeout, "request-timeout", client.RequestTimeout, "Kafka request timeout")
	fs.BoolVar(&client.TLSEnabled, "tls", false, "enable TLS")
	fs.BoolVar(&client.TLSInsecure, "tls-insecure", false, "skip TLS certificate verification (unsafe)")
	fs.StringVar(&client.TLSCA, "tls-ca", "", "PEM CA certificate path")
	fs.StringVar(&client.TLSCert, "tls-cert", "", "PEM client certificate path")
	fs.StringVar(&client.TLSKey, "tls-key", "", "PEM client private key path")
	fs.StringVar(&client.SASLMechanism, "sasl-mechanism", "", "plain, scram-sha-256, or scram-sha-512")
	fs.StringVar(&username, "username", "", "SASL username; stored in the system credential store")
	fs.BoolVar(&passwordStdin, "password-stdin", false, "read the SASL password from stdin")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if strings.TrimSpace(name) == "" {
		fmt.Fprintln(stderr, "--name is required")
		return 2
	}
	password := ""
	if passwordStdin {
		input, err := io.ReadAll(io.LimitReader(stdin, 1024*1024+1))
		if err != nil {
			fmt.Fprintln(stderr, "read password:", err)
			return 1
		}
		if len(input) > 1024*1024 {
			fmt.Fprintln(stderr, "password is too large")
			return 2
		}
		password = strings.TrimRight(string(input), "\r\n")
	} else if value, ok := os.LookupEnv("FRANZCTL_SASL_PASSWORD"); ok {
		password = value
	}
	if username == "" {
		username = os.Getenv("FRANZCTL_SASL_USERNAME")
	}
	client.SASLUsername = username
	client.SASLPassword = password
	if err := client.Validate(); err != nil {
		fmt.Fprintln(stderr, "configuration:", err)
		return 2
	}
	credentials := config.Credentials{Username: username, Password: password}
	if client.SASLMechanism != "" && credentials.Password == "" {
		fmt.Fprintln(stderr, "--password-stdin or FRANZCTL_SASL_PASSWORD is required when SASL is enabled")
		return 2
	}
	profile, err := config.NewConnectionProfile(name, client)
	if err != nil {
		fmt.Fprintln(stderr, "create connection:", err)
		return 2
	}
	profile.CredentialsStored = !credentials.Empty()
	if profile.CredentialsStored {
		if err := secrets.Set(profile.ID, credentials); err != nil {
			fmt.Fprintln(stderr, "store credentials:", err)
			return 1
		}
	}
	if err := store.Save(profile); err != nil {
		if profile.CredentialsStored {
			_ = secrets.Delete(profile.ID)
		}
		fmt.Fprintln(stderr, "save connection:", err)
		return 1
	}
	if _, err := store.Use(profile.ID); err != nil {
		fmt.Fprintln(stderr, "select connection:", err)
		return 1
	}
	fmt.Fprintf(stdout, "Saved and selected connection %q.\n", profile.Name)
	return 0
}

func removeConnection(
	store *config.ProfileStore,
	secrets config.SecretStore,
	args []string,
	stdout, stderr io.Writer,
) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: franzctl connection remove <name>")
		return 2
	}
	profile, err := store.Delete(args[0])
	if err != nil {
		fmt.Fprintln(stderr, "remove connection:", err)
		return 1
	}
	if profile.CredentialsStored {
		if err := secrets.Delete(profile.ID); err != nil {
			fmt.Fprintln(stderr, "remove saved credentials:", err)
			return 1
		}
	}
	fmt.Fprintf(stdout, "Removed connection %q.\n", profile.Name)
	return 0
}
