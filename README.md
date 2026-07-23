# franzctl

Keyboard-first terminal workbench and scriptable CLI for Apache Kafka.

`franzctl` is built for engineers who live in the terminal and want one tool
for browsing topics, inspecting records, producing events, and composing data
codecs. The interactive experience follows the same panel-oriented design
language as [wtui](https://github.com/D1ssolve/wtui); the command interface
remains available for shell scripts and CI.

> Independent community project powered by
> [franz-go](https://github.com/twmb/franz-go). It is not affiliated with or
> maintained by the franz-go project.

## What it looks like

```text
┌──────────────────────┐ ┌────────────────────────────────┐ ┌──────────────────────────┐
│ [1] Topics           │ │ [2] Records — orders.created  │ │ [3] Inspector            │
│                      │ │                                │ │                          │
│ › orders.created p:6 │ │ › 0:1842 {"id":"A-42"}        │ │ partition  0             │
│   orders.failed  p:3 │ │   1:902  {"id":"B-17"}        │ │ offset     1842          │
│   payments       p:6 │ │   0:1843 {"id":"C-03"}        │ │ key        order-A-42    │
│                      │ │                                │ │ value      { ... }       │
└──────────────────────┘ └────────────────────────────────┘ └──────────────────────────┘
┌──────────────────────────────────────────────────────────────────────────────────────┐
│ [0] Output  Loaded 3 topics. Loaded 100 records from orders.created.                 │
└──────────────────────────────────────────────────────────────────────────────────────┘
```

## Features

- Bubble Tea TUI with focused Topics, Records, Inspector, and Output panels
- create topics and publish records without leaving the TUI
- standalone `topic create`, `produce`, and `consume` commands
- partition and offset-aware record browsing
- JSONL, pretty JSON, and raw output for automation
- composable codec pipelines: `json,gzip`, `base64`, `hex`, numeric types
- custom codecs through direct executable invocation, without a shell
- TLS, mTLS, SASL/PLAIN, and SASL/SCRAM
- first-run connection setup and a reusable history of Kafka clusters
- encrypted credential storage through macOS Keychain, Linux Secret Service,
  or Windows DPAPI
- static Go binaries and GoReleaser packaging for macOS, Linux, and Windows
- Homebrew Cask publishing through `D1ssolve/homebrew-tap`

## Installation

Build from source:

```bash
git clone https://github.com/D1ssolve/franzctl.git
cd franzctl
make build
./bin/franzctl
```

Or install the Go binary:

```bash
go install github.com/D1ssolve/franzctl/cmd/franzctl@latest
```

Homebrew will be available after the first tagged release:

```bash
brew install D1ssolve/tap/franzctl
```

## Quick start

Start the included Kafka-compatible Redpanda broker:

```bash
docker compose up -d
franzctl
```

On the first launch, the connection wizard asks for a profile name, bootstrap
brokers, TLS, and optional SASL credentials. Future launches start with the
saved connection picker. Credentials are masked while typing and are never
written to the profile file.

TUI keys:

| Key | Action |
|---|---|
| `1` / `2` | focus Topics or Records |
| `j` / `k` | move selection |
| `Enter` | open records for the selected topic |
| `n` | create a topic |
| `p` | produce a record to the selected topic |
| `r` | refresh topic metadata |
| `?` | show help |
| `q` | quit |

## Scriptable CLI

Create a topic:

```bash
franzctl topic create \
  --broker localhost:9092 \
  --topic events \
  --partitions 3 \
  --replication-factor 1
```

Produce JSON:

```bash
franzctl produce \
  --broker localhost:9092 \
  --topic events \
  --key user-42 \
  --value '{"type":"user.created","id":42}' \
  --value-codec json
```

Consume ten records:

```bash
franzctl consume \
  --broker localhost:9092 \
  --topic events \
  --from beginning \
  --max 10 \
  --follow=false \
  --value-codec json
```

Saved connections work with every command:

```bash
# Local broker without authentication
franzctl connection add \
  --name local \
  --broker localhost:9092

# Managed or third-party broker
printf '%s' "$KAFKA_PASSWORD" | franzctl connection add \
  --name production \
  --broker broker-1.example.com:9093 \
  --broker broker-2.example.com:9093 \
  --tls \
  --sasl-mechanism scram-sha-512 \
  --username alice \
  --password-stdin

franzctl connection list
franzctl connection use production
franzctl consume --topic events --from beginning

# Select a profile for one command without changing the active profile.
franzctl consume --connection local --topic events
```

## Codec pipelines

A pipeline runs left-to-right when producing and in reverse when consuming:

```bash
franzctl produce ... --value '{"n":1}' --value-codec json,gzip
franzctl consume ... --value-codec json,gzip
```

Built-in stages:

```text
bytes string json base64 hex int64 uint64 float64 bool gzip
```

External codecs use `exec:/absolute/path/to/codec`. The program reads bytes
from stdin, writes bytes to stdout, and receives `FRANZCTL_CODEC_MODE=encode`
or `FRANZCTL_CODEC_MODE=decode`. Processes are launched directly; codec specs
are never interpreted by a shell.

## Connection profiles and credentials

Connection profiles contain broker addresses, TLS settings, SASL mechanism,
and usage timestamps. They are stored in the operating system's user
configuration directory under `franzctl/connections.json` with owner-only
permissions. The file never contains usernames or passwords.

Credentials are stored separately:

| Platform | Protected storage |
|---|---|
| macOS | Login Keychain |
| Linux | Secret Service via `secret-tool` (`libsecret-tools`) |
| Windows | DPAPI encrypted for the current Windows user |

There is deliberately no plaintext fallback. On Linux, install
`libsecret-tools` before saving an authenticated profile. Profiles without
SASL credentials do not require a credential-store service.

Useful commands:

```text
franzctl connection add --help
franzctl connection list [--json]
franzctl connection show <name>
franzctl connection use <name>
franzctl connection remove <name>
```

Environment variables remain available for ephemeral use and override the
selected profile:

| Variable | Purpose |
|---|---|
| `FRANZCTL_BROKERS` | comma-separated bootstrap brokers; defaults to `localhost:9092` |
| `FRANZCTL_CONNECTION` | saved profile to use |
| `FRANZCTL_CLIENT_ID` | Kafka client ID |
| `FRANZCTL_TLS` | enable TLS |
| `FRANZCTL_SASL_MECHANISM` | `plain`, `scram-sha-256`, or `scram-sha-512` |
| `FRANZCTL_SASL_USERNAME` | SASL username |
| `FRANZCTL_SASL_PASSWORD` | SASL password |

CLI commands accept the same settings as flags. Use `--password-stdin` when
saving a profile; a password flag is intentionally not provided because
command arguments may be visible to other local processes.

## Architecture

The project uses the same boundary-first structure as `wtui`, adapted to
Kafka:

- `cmd/franzctl` — binary entrypoint, build metadata, TUI/CLI dispatch
- `internal/app` — composition root and scriptable CLI orchestration
- `internal/domain` — behavior-free topic and record models
- `internal/kafka` — Kafka manager interface and franz-go adapter
- `internal/tui` — Bubble Tea model, commands, dialogs, and layout
- `internal/tui/panels` — focused panel components and panel messages
- `internal/config` — Kafka, TLS, and SASL connection configuration
  plus profile history and platform credential stores
- `internal/codec` — built-in and executable codec pipelines
- `internal/record` — safe JSON representation for text and binary payloads

Dependency direction:

```text
cmd/franzctl → internal/app → internal/kafka → franz-go
            ↘ internal/tui → manager interface + domain models
```

The TUI does not call franz-go directly. Kafka operations stay behind
`kafka.Manager`, which makes panel behavior testable without a broker.

## Development

```bash
make fmt
make vet
make test
make build
docker compose up -d
make smoke
```

Go 1.24.2 or newer is required.

## Current scope

This first version intentionally supports the core daily workflow: list and
create topics, inspect a bounded record snapshot, produce records, and use the
full streaming CLI. Topic alteration/deletion, Schema Registry integration,
consumer-group management, and live tailing inside the TUI are planned
extensions rather than copied Kaskade screens.

## License

Apache-2.0
