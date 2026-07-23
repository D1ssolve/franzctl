package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

func LoadForArgs(args []string) (Client, error) {
	if hasArgument(args, "-h") || hasArgument(args, "--help") {
		client := FromEnv()
		prepareBrokerFlags(&client, args)
		return client, nil
	}
	reference := connectionArgument(args)
	if reference == "" {
		reference = strings.TrimSpace(os.Getenv("FRANZCTL_CONNECTION"))
	}

	explicitBrokers := strings.TrimSpace(os.Getenv("FRANZCTL_BROKERS")) != ""
	if reference == "" && (explicitBrokers || hasBrokerArgument(args)) {
		client := FromEnv()
		prepareBrokerFlags(&client, args)
		return client, nil
	}

	store, err := DefaultProfileStore()
	if err != nil {
		if reference != "" {
			return Client{}, err
		}
		client := FromEnv()
		prepareBrokerFlags(&client, args)
		return client, nil
	}

	var profile ConnectionProfile
	if reference != "" {
		profile, err = store.Resolve(reference)
	} else {
		profile, err = store.Current()
	}
	if errors.Is(err, ErrConnectionNotFound) && reference == "" {
		client := FromEnv()
		prepareBrokerFlags(&client, args)
		return client, nil
	}
	if err != nil {
		return Client{}, err
	}
	client, err := ClientFromProfile(store, NewSecretStore(store), profile)
	if err != nil {
		return Client{}, err
	}
	overlayEnvironment(&client)
	prepareBrokerFlags(&client, args)
	return client, nil
}

func ClientFromProfile(store *ProfileStore, secrets SecretStore, profile ConnectionProfile) (Client, error) {
	var credentials Credentials
	if profile.CredentialsStored {
		stored, err := secrets.Get(profile.ID)
		if err != nil {
			if errors.Is(err, ErrSecretNotFound) {
				return Client{}, fmt.Errorf(
					"connection %q expects saved credentials, but they are missing; add the connection again",
					profile.Name,
				)
			}
			return Client{}, fmt.Errorf("load credentials for connection %q: %w", profile.Name, err)
		}
		credentials = stored
	}
	client, err := profile.Client(credentials)
	if err != nil {
		return Client{}, err
	}
	client.Connection = profile.Name
	return client, nil
}

func connectionArgument(args []string) string {
	for i, arg := range args {
		if value, ok := strings.CutPrefix(arg, "--connection="); ok {
			return strings.TrimSpace(value)
		}
		if arg == "--connection" && i+1 < len(args) {
			return strings.TrimSpace(args[i+1])
		}
	}
	return ""
}

func prepareBrokerFlags(client *Client, args []string) {
	if hasBrokerArgument(args) {
		client.Brokers = nil
	}
}

func hasBrokerArgument(args []string) bool {
	for _, arg := range args {
		if arg == "--broker" || strings.HasPrefix(arg, "--broker=") {
			return true
		}
	}
	return false
}

func hasArgument(args []string, wanted string) bool {
	for _, arg := range args {
		if arg == wanted {
			return true
		}
	}
	return false
}

func overlayEnvironment(client *Client) {
	if value := strings.TrimSpace(os.Getenv("FRANZCTL_BROKERS")); value != "" {
		client.Brokers = nil
		_ = client.Brokers.Set(value)
	}
	if value := os.Getenv("FRANZCTL_CLIENT_ID"); value != "" {
		client.ClientID = value
	}
	if _, ok := os.LookupEnv("FRANZCTL_TLS"); ok {
		client.TLSEnabled = envBool("FRANZCTL_TLS")
	}
	if value := os.Getenv("FRANZCTL_SASL_MECHANISM"); value != "" {
		client.SASLMechanism = value
	}
	if value, ok := os.LookupEnv("FRANZCTL_SASL_USERNAME"); ok {
		client.SASLUsername = value
	}
	if value, ok := os.LookupEnv("FRANZCTL_SASL_PASSWORD"); ok {
		client.SASLPassword = value
	}
}
