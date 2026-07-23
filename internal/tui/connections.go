package tui

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/D1ssolve/franzctl/internal/config"
)

type ConnectionSelection struct {
	Profile     config.ConnectionProfile
	Credentials config.Credentials
	Created     bool
}

type ConnectionSelector struct {
	profiles []config.ConnectionProfile
	cursor   int
	creating bool
	inputs   []textinput.Model
	index    int
	result   *ConnectionSelection
	err      string
	cancel   bool
	styles   Styles
	defaults config.Client
}

func NewConnectionSelector(
	profiles []config.ConnectionProfile,
	currentID string,
	defaults ...config.Client,
) ConnectionSelector {
	model := ConnectionSelector{
		profiles: append([]config.ConnectionProfile(nil), profiles...),
		styles:   NewStyles(),
	}
	if len(defaults) > 0 {
		model.defaults = defaults[0]
	}
	for i, profile := range model.profiles {
		if profile.ID == currentID {
			model.cursor = i
			break
		}
	}
	if len(model.profiles) == 0 {
		model.startCreate()
	}
	return model
}

func (m ConnectionSelector) Init() tea.Cmd {
	if m.creating && len(m.inputs) > 0 {
		return m.inputs[m.index].Focus()
	}
	return nil
}

func (m ConnectionSelector) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if m.creating {
		return m.updateCreate(key)
	}

	switch key.String() {
	case "q", "ctrl+c", "esc":
		m.cancel = true
		return m, tea.Quit
	case "j", "down":
		if len(m.profiles) > 0 {
			m.cursor = (m.cursor + 1) % len(m.profiles)
		}
	case "k", "up":
		if len(m.profiles) > 0 {
			m.cursor--
			if m.cursor < 0 {
				m.cursor = len(m.profiles) - 1
			}
		}
	case "n":
		m.startCreate()
		return m, m.inputs[0].Focus()
	case "enter":
		if len(m.profiles) > 0 {
			m.result = &ConnectionSelection{Profile: m.profiles[m.cursor]}
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m ConnectionSelector) View() string {
	if m.creating {
		return m.createView()
	}
	var body strings.Builder
	body.WriteString(m.styles.Header.Render("franzctl · connections"))
	body.WriteString("\n\n")
	body.WriteString("Choose a saved Kafka connection:\n\n")
	for i, profile := range m.profiles {
		cursor := "  "
		if i == m.cursor {
			cursor = "› "
		}
		auth := "no auth"
		if profile.SASLMechanism != "" {
			auth = profile.SASLMechanism
		}
		line := fmt.Sprintf(
			"%s%-18s %s  tls=%t  %s",
			cursor,
			profile.Name,
			strings.Join(profile.Brokers, ","),
			profile.TLSEnabled,
			auth,
		)
		if i == m.cursor {
			body.WriteString(m.styles.Active.Render(line))
		} else {
			body.WriteString(line)
		}
		body.WriteByte('\n')
	}
	body.WriteString("\n")
	body.WriteString(m.styles.Muted.Render("[j/k] select  [Enter] connect  [n] new connection  [q] quit"))
	return lipgloss.Place(100, 28, lipgloss.Center, lipgloss.Center, m.styles.Modal.Width(92).Render(body.String()))
}

func (m ConnectionSelector) Selection() (ConnectionSelection, error) {
	if m.cancel {
		return ConnectionSelection{}, errors.New("connection selection cancelled")
	}
	if m.result == nil {
		return ConnectionSelection{}, errors.New("no connection selected")
	}
	return *m.result, nil
}

func (m *ConnectionSelector) startCreate() {
	m.creating = true
	m.index = 0
	m.err = ""
	brokers := "localhost:9092"
	if len(m.defaults.Brokers) > 0 {
		brokers = strings.Join(m.defaults.Brokers, ",")
	}
	tlsEnabled := strconv.FormatBool(m.defaults.TLSEnabled)
	m.inputs = []textinput.Model{
		connectionInput("name", ""),
		connectionInput("brokers (comma-separated)", brokers),
		connectionInput("TLS (true/false)", tlsEnabled),
		connectionInput("SASL mechanism (optional)", m.defaults.SASLMechanism),
		connectionInput("username (stored securely)", m.defaults.SASLUsername),
		connectionPasswordInput("password (stored securely)"),
	}
	m.inputs[5].SetValue(m.defaults.SASLPassword)
}

func (m ConnectionSelector) updateCreate(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "ctrl+c":
		m.cancel = true
		return m, tea.Quit
	case "esc":
		if len(m.profiles) == 0 {
			m.cancel = true
			return m, tea.Quit
		}
		m.creating = false
		m.inputs = nil
		m.err = ""
		return m, nil
	case "tab", "shift+tab", "up", "down":
		m.inputs[m.index].Blur()
		if key.String() == "shift+tab" || key.String() == "up" {
			m.index--
		} else {
			m.index++
		}
		if m.index < 0 {
			m.index = len(m.inputs) - 1
		}
		m.index %= len(m.inputs)
		return m, m.inputs[m.index].Focus()
	case "enter":
		selection, err := m.buildConnection()
		if err != nil {
			m.err = err.Error()
			return m, nil
		}
		m.result = &selection
		return m, tea.Quit
	}
	var cmd tea.Cmd
	m.inputs[m.index], cmd = m.inputs[m.index].Update(key)
	return m, cmd
}

func (m ConnectionSelector) buildConnection() (ConnectionSelection, error) {
	tlsEnabled, err := strconv.ParseBool(strings.TrimSpace(m.inputs[2].Value()))
	if err != nil {
		return ConnectionSelection{}, errors.New("TLS must be true or false")
	}
	client := config.Client{
		ClientID:       "franzctl",
		RequestTimeout: 15 * time.Second,
		TLSEnabled:     tlsEnabled,
		SASLMechanism:  strings.ToLower(strings.TrimSpace(m.inputs[3].Value())),
		SASLUsername:   m.inputs[4].Value(),
		SASLPassword:   m.inputs[5].Value(),
	}
	if err := client.Brokers.Set(m.inputs[1].Value()); err != nil {
		return ConnectionSelection{}, err
	}
	if err := client.Validate(); err != nil {
		return ConnectionSelection{}, err
	}
	if client.SASLMechanism != "" && client.SASLPassword == "" {
		return ConnectionSelection{}, errors.New("password is required when SASL is enabled")
	}
	profile, err := config.NewConnectionProfile(m.inputs[0].Value(), client)
	if err != nil {
		return ConnectionSelection{}, err
	}
	credentials := config.Credentials{
		Username: client.SASLUsername,
		Password: client.SASLPassword,
	}
	profile.CredentialsStored = !credentials.Empty()
	return ConnectionSelection{
		Profile:     profile,
		Credentials: credentials,
		Created:     true,
	}, nil
}

func (m ConnectionSelector) createView() string {
	var body strings.Builder
	body.WriteString(m.styles.Header.Render("New Kafka connection"))
	body.WriteString("\n\n")
	body.WriteString(m.styles.Muted.Render("The profile is saved locally; credentials go to the operating system's secure store."))
	body.WriteString("\n\n")
	for i, input := range m.inputs {
		label := "  " + input.Placeholder
		if i == m.index {
			label = "● " + input.Placeholder
		}
		body.WriteString(m.styles.Muted.Render(label))
		body.WriteByte('\n')
		body.WriteString(input.View())
		body.WriteString("\n\n")
	}
	if m.err != "" {
		body.WriteString(m.styles.Error.Render(m.err))
		body.WriteByte('\n')
	}
	body.WriteString(m.styles.Muted.Render("[Tab] next  [Enter] save and connect  [Esc] back"))
	return lipgloss.Place(100, 34, lipgloss.Center, lipgloss.Center, m.styles.Modal.Width(76).Render(body.String()))
}

func connectionInput(placeholder, value string) textinput.Model {
	input := textinput.New()
	input.Prompt = "› "
	input.Placeholder = placeholder
	input.SetValue(value)
	input.CharLimit = 4096
	return input
}

func connectionPasswordInput(placeholder string) textinput.Model {
	input := connectionInput(placeholder, "")
	input.EchoMode = textinput.EchoPassword
	input.EchoCharacter = '•'
	return input
}
