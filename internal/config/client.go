package config

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"github.com/twmb/franz-go/pkg/sasl/scram"
)

type Strings []string

func (s *Strings) String() string { return strings.Join(*s, ",") }

func (s *Strings) Set(value string) error {
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			*s = append(*s, item)
		}
	}
	return nil
}

type Client struct {
	Brokers        Strings
	ClientID       string
	RequestTimeout time.Duration
	TLSEnabled     bool
	TLSInsecure    bool
	TLSCA          string
	TLSCert        string
	TLSKey         string
	SASLMechanism  string
	SASLUsername   string
	SASLPassword   string
	AllowAutoTopic bool
}

func FromEnv() Client {
	client := Client{
		ClientID:       envOr("FRANZCTL_CLIENT_ID", "franzctl"),
		RequestTimeout: 15 * time.Second,
		TLSEnabled:     envBool("FRANZCTL_TLS"),
		SASLMechanism:  envOr("FRANZCTL_SASL_MECHANISM", ""),
		SASLUsername:   os.Getenv("FRANZCTL_SASL_USERNAME"),
		SASLPassword:   os.Getenv("FRANZCTL_SASL_PASSWORD"),
	}
	brokers := envOr("FRANZCTL_BROKERS", "localhost:9092")
	_ = client.Brokers.Set(brokers)
	return client
}

func (c *Client) Bind(fs *flag.FlagSet) {
	defaults := FromEnv()
	if len(c.Brokers) == 0 {
		if brokers := os.Getenv("FRANZCTL_BROKERS"); brokers != "" {
			_ = c.Brokers.Set(brokers)
		}
	}
	if c.ClientID == "" {
		c.ClientID = defaults.ClientID
	}
	if c.RequestTimeout == 0 {
		c.RequestTimeout = defaults.RequestTimeout
	}
	if c.SASLMechanism == "" {
		c.SASLMechanism = defaults.SASLMechanism
	}
	if c.SASLUsername == "" {
		c.SASLUsername = defaults.SASLUsername
	}
	if c.SASLPassword == "" {
		c.SASLPassword = defaults.SASLPassword
	}
	fs.Var(&c.Brokers, "broker", "Kafka bootstrap broker; repeat or use a comma-separated list")
	fs.StringVar(&c.ClientID, "client-id", c.ClientID, "Kafka client ID")
	fs.DurationVar(&c.RequestTimeout, "request-timeout", c.RequestTimeout, "Kafka request timeout")
	fs.BoolVar(&c.TLSEnabled, "tls", c.TLSEnabled, "enable TLS")
	fs.BoolVar(&c.TLSInsecure, "tls-insecure", false, "skip TLS certificate verification (unsafe)")
	fs.StringVar(&c.TLSCA, "tls-ca", "", "PEM CA certificate path")
	fs.StringVar(&c.TLSCert, "tls-cert", "", "PEM client certificate path")
	fs.StringVar(&c.TLSKey, "tls-key", "", "PEM client private key path")
	fs.StringVar(&c.SASLMechanism, "sasl-mechanism", c.SASLMechanism, "SASL mechanism: plain, scram-sha-256, scram-sha-512")
	fs.StringVar(&c.SASLUsername, "sasl-username", c.SASLUsername, "SASL username")
	fs.StringVar(&c.SASLPassword, "sasl-password", c.SASLPassword, "SASL password (prefer the environment variable)")
	fs.BoolVar(&c.AllowAutoTopic, "allow-auto-topic-creation", false, "allow broker-side automatic topic creation")
}

func (c Client) Validate() error {
	if len(c.Brokers) == 0 {
		return errors.New("at least one --broker is required (or set FRANZCTL_BROKERS)")
	}
	if c.RequestTimeout <= 0 {
		return errors.New("--request-timeout must be positive")
	}
	if (c.TLSCert == "") != (c.TLSKey == "") {
		return errors.New("--tls-cert and --tls-key must be set together")
	}
	switch strings.ToLower(c.SASLMechanism) {
	case "", "plain", "scram-sha-256", "scram-sha-512":
	default:
		return fmt.Errorf("unsupported SASL mechanism %q", c.SASLMechanism)
	}
	if c.SASLMechanism != "" && c.SASLUsername == "" {
		return errors.New("--sasl-username is required when SASL is enabled")
	}
	return nil
}

func (c Client) Options(extra ...kgo.Opt) ([]kgo.Opt, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}

	opts := []kgo.Opt{
		kgo.SeedBrokers(c.Brokers...),
		kgo.ClientID(c.ClientID),
		kgo.RequestTimeoutOverhead(c.RequestTimeout),
		kgo.AllowAutoTopicCreation(c.AllowAutoTopic),
	}

	if c.TLSEnabled || c.TLSInsecure || c.TLSCA != "" || c.TLSCert != "" {
		tlsConfig, err := c.tlsConfig()
		if err != nil {
			return nil, err
		}
		opts = append(opts, kgo.DialTLSConfig(tlsConfig))
	}

	auth := plain.Auth{User: c.SASLUsername, Pass: c.SASLPassword}
	switch strings.ToLower(c.SASLMechanism) {
	case "plain":
		opts = append(opts, kgo.SASL(auth.AsMechanism()))
	case "scram-sha-256":
		opts = append(opts, kgo.SASL(scram.Auth{User: c.SASLUsername, Pass: c.SASLPassword}.AsSha256Mechanism()))
	case "scram-sha-512":
		opts = append(opts, kgo.SASL(scram.Auth{User: c.SASLUsername, Pass: c.SASLPassword}.AsSha512Mechanism()))
	}

	return append(opts, extra...), nil
}

func (c Client) tlsConfig() (*tls.Config, error) {
	cfg := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: c.TLSInsecure, // #nosec G402: explicitly requested by the operator.
	}
	if c.TLSCA != "" {
		pem, err := os.ReadFile(c.TLSCA)
		if err != nil {
			return nil, fmt.Errorf("read TLS CA: %w", err)
		}
		pool, err := x509.SystemCertPool()
		if err != nil || pool == nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM(pem) {
			return nil, errors.New("TLS CA file contains no valid certificates")
		}
		cfg.RootCAs = pool
	}
	if c.TLSCert != "" {
		cert, err := tls.LoadX509KeyPair(c.TLSCert, c.TLSKey)
		if err != nil {
			return nil, fmt.Errorf("load TLS client certificate: %w", err)
		}
		cfg.Certificates = []tls.Certificate{cert}
	}
	return cfg, nil
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envBool(key string) bool {
	switch strings.ToLower(os.Getenv(key)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
