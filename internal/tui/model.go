package tui

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/D1ssolve/franzctl/internal/domain"
	"github.com/D1ssolve/franzctl/internal/kafka"
	"github.com/D1ssolve/franzctl/internal/tui/panels"
)

type Model struct {
	manager kafka.Manager
	brokers []string
	version string

	width  int
	height int
	ready  bool
	focus  FocusPanel

	topics    panels.TopicsPanel
	records   panels.RecordsPanel
	inspector panels.InspectorPanel
	output    panels.OutputPanel

	dialog *dialog
	help   bool
	busy   bool
	styles Styles
}

func New(manager kafka.Manager, brokers []string, version string) (Model, error) {
	if manager == nil {
		return Model{}, fmt.Errorf("tui.New: manager must not be nil")
	}
	model := Model{
		manager: manager,
		brokers: append([]string(nil), brokers...),
		version: version,
		focus:   FocusTopics,
		topics:  panels.NewTopicsPanel(30, 20),
		records: panels.NewRecordsPanel(50, 20),
		inspector: panels.NewInspectorPanel(30, 20),
		output: panels.NewOutputPanel(110, 8),
		styles: NewStyles(),
	}
	model.setFocus(FocusTopics)
	return model, nil
}

func (m Model) Init() tea.Cmd {
	return loadTopicsCmd(m.manager)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height, m.ready = msg.Width, msg.Height, true
		m.resize()
		return m, nil
	case tea.KeyMsg:
		if m.dialog != nil {
			return m, m.dialog.update(msg)
		}
		if m.help {
			m.help = false
			return m, nil
		}
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "?":
			m.help = true
			return m, nil
		case "tab":
			if m.focus == FocusTopics {
				m.setFocus(FocusRecords)
			} else {
				m.setFocus(FocusTopics)
			}
			return m, nil
		case "1":
			m.setFocus(FocusTopics)
			return m, nil
		case "2":
			m.setFocus(FocusRecords)
			return m, nil
		case "r":
			m.busy = true
			m.output.Append("Refreshing topic metadata...")
			return m, loadTopicsCmd(m.manager)
		}

		if m.focus == FocusTopics {
			panel, cmd := m.topics.Update(msg)
			m.topics = panel
			return m, cmd
		}
		panel, cmd := m.records.Update(msg)
		m.records = panel
		return m, cmd

	case panels.FocusTopicsMsg:
		m.setFocus(FocusTopics)
		return m, nil
	case panels.FocusRecordsMsg:
		m.setFocus(FocusRecords)
		return m, nil
	case panels.TopicSelectedMsg:
		m.busy = true
		m.output.Append("Loading records from " + msg.Topic + "...")
		return m, loadRecordsCmd(m.manager, msg.Topic)
	case panels.RecordSelectedMsg:
		m.inspector.SetRecord(msg.Record)
		return m, nil
	case panels.OpenCreateTopicMsg:
		m.dialog = newCreateTopicDialog()
		return m, m.dialog.init()
	case panels.OpenProduceMsg:
		m.dialog = newProduceDialog(msg.Topic)
		return m, m.dialog.init()
	case closeDialogMsg:
		m.dialog = nil
		return m, nil
	case submitCreateTopicMsg:
		m.dialog = nil
		m.busy = true
		m.output.Append("Creating topic " + msg.Params.Name + "...")
		return m, createTopicCmd(m.manager, msg.Params)
	case submitProduceMsg:
		m.dialog = nil
		m.busy = true
		m.output.Append("Producing record to " + msg.Params.Topic + "...")
		return m, produceCmd(m.manager, msg.Params)
	case topicsLoadedMsg:
		m.busy = false
		if msg.Err != nil {
			m.output.Append("Metadata error: " + msg.Err.Error())
			return m, nil
		}
		m.topics.SetTopics(msg.Topics)
		m.output.Append(fmt.Sprintf("Loaded %d topics.", len(msg.Topics)))
		if selected := m.topics.Selected(); selected != nil {
			return m, loadRecordsCmd(m.manager, selected.Name)
		}
		return m, nil
	case recordsLoadedMsg:
		m.busy = false
		if msg.Err != nil {
			m.output.Append("Consume error: " + msg.Err.Error())
			return m, nil
		}
		m.records.SetRecords(msg.Topic, msg.Records)
		m.inspector.SetRecord(m.records.Selected())
		m.output.Append(fmt.Sprintf("Loaded %d records from %s.", len(msg.Records), msg.Topic))
		return m, nil
	case operationDoneMsg:
		m.busy = false
		if msg.Err != nil {
			m.output.Append(msg.Operation + " failed: " + msg.Err.Error())
			return m, nil
		}
		m.output.Append(msg.Operation + " complete.")
		if msg.Record != nil {
			topic := msg.Record.Topic
			return m, loadRecordsCmd(m.manager, topic)
		}
		return m, loadTopicsCmd(m.manager)
	}
	return m, nil
}

func (m Model) View() string {
	if !m.ready {
		return "Starting franzctl…"
	}

	header := m.styles.Header.Render("franzctl") + "  " +
		m.styles.Muted.Render("Kafka workbench · "+joinBrokers(m.brokers)+" · "+m.version)
	top := lipgloss.JoinHorizontal(lipgloss.Top,
		m.topics.View(), m.records.View(), m.inspector.View(),
	)
	footer := m.styles.Footer.Render(m.footer())
	base := lipgloss.JoinVertical(lipgloss.Left, header, top, m.output.View(), footer)

	if m.dialog != nil {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.dialog.view(m.styles))
	}
	if m.help {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.helpView())
	}
	return base
}

func (m *Model) resize() {
	headerHeight, outputHeight, footerHeight := 1, 7, 1
	topHeight := max(5, m.height-headerHeight-outputHeight-footerHeight)
	topicsWidth := max(24, m.width*28/100)
	inspectorWidth := max(28, m.width*30/100)
	recordsWidth := max(30, m.width-topicsWidth-inspectorWidth)

	m.topics.SetSize(topicsWidth, topHeight)
	m.records.SetSize(recordsWidth, topHeight)
	m.inspector.SetSize(inspectorWidth, topHeight)
	m.output.SetSize(m.width, outputHeight)
}

func (m *Model) setFocus(focus FocusPanel) {
	m.focus = focus
	m.topics.SetFocused(focus == FocusTopics)
	m.records.SetFocused(focus == FocusRecords)
}

func (m Model) footer() string {
	busy := ""
	if m.busy {
		busy = "working…  "
	}
	if m.focus == FocusTopics {
		return busy + "[j/k] select  [Enter] records  [n] new topic  [r] refresh  [?] help  [q] quit"
	}
	return busy + "[j/k] select  [p] produce  [Esc] topics  [r] refresh  [?] help  [q] quit"
}

func (m Model) helpView() string {
	body := m.styles.Header.Render("franzctl keys") + "\n\n" +
		"[1]/[2]     focus Topics or Records\n" +
		"[j]/[k]     move selection\n" +
		"[n]         create a topic\n" +
		"[p]         produce to selected topic\n" +
		"[r]         refresh metadata\n" +
		"[q]         quit\n\n" +
		m.styles.Muted.Render("Press any key to close")
	return m.styles.Modal.Width(52).Render(body)
}

func loadTopicsCmd(manager kafka.Manager) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		topics, err := manager.ListTopics(ctx)
		return topicsLoadedMsg{Topics: topics, Err: err}
	}
}

func loadRecordsCmd(manager kafka.Manager, topic string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2500*time.Millisecond)
		defer cancel()
		records, err := manager.ReadRecords(ctx, topic, 100)
		return recordsLoadedMsg{Topic: topic, Records: records, Err: err}
	}
}

func createTopicCmd(manager kafka.Manager, params domain.CreateTopicParams) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		err := manager.CreateTopic(ctx, params)
		return operationDoneMsg{Operation: "Create topic " + params.Name, Err: err}
	}
}

func produceCmd(manager kafka.Manager, params domain.ProduceParams) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		record, err := manager.Produce(ctx, params)
		if err != nil {
			return operationDoneMsg{Operation: "Produce to " + params.Topic, Err: err}
		}
		return operationDoneMsg{Operation: "Produce to " + params.Topic, Record: &record}
	}
}

func joinBrokers(brokers []string) string {
	if len(brokers) == 0 {
		return "no brokers"
	}
	result := brokers[0]
	for _, broker := range brokers[1:] {
		result += "," + broker
	}
	return result
}
