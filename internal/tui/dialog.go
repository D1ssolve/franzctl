package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/D1ssolve/franzctl/internal/domain"
)

type dialogKind int

const (
	dialogCreateTopic dialogKind = iota
	dialogProduce
)

type dialog struct {
	kind   dialogKind
	topic  string
	inputs []textinput.Model
	index  int
	err    string
}

type submitCreateTopicMsg struct {
	Params domain.CreateTopicParams
}

type submitProduceMsg struct {
	Params domain.ProduceParams
}

type closeDialogMsg struct{}

func newCreateTopicDialog() *dialog {
	return &dialog{
		kind: dialogCreateTopic,
		inputs: []textinput.Model{
			newInput("topic name", ""),
			newInput("partitions", "1"),
			newInput("replication factor", "1"),
		},
	}
}

func newProduceDialog(topic string) *dialog {
	return &dialog{
		kind:  dialogProduce,
		topic: topic,
		inputs: []textinput.Model{
			newInput("key (optional)", ""),
			newInput("value", ""),
		},
	}
}

func newInput(placeholder, value string) textinput.Model {
	input := textinput.New()
	input.Prompt = "› "
	input.Placeholder = placeholder
	input.SetValue(value)
	input.CharLimit = 1024 * 1024
	return input
}

func (d *dialog) init() tea.Cmd {
	d.inputs[0].Focus()
	return textinput.Blink
}

func (d *dialog) update(msg tea.Msg) tea.Cmd {
	key, ok := msg.(tea.KeyMsg)
	if ok {
		switch key.String() {
		case "esc":
			return func() tea.Msg { return closeDialogMsg{} }
		case "tab", "shift+tab", "up", "down":
			d.inputs[d.index].Blur()
			if key.String() == "shift+tab" || key.String() == "up" {
				d.index--
			} else {
				d.index++
			}
			if d.index < 0 {
				d.index = len(d.inputs) - 1
			}
			d.index %= len(d.inputs)
			return d.inputs[d.index].Focus()
		case "enter":
			return d.submit()
		}
	}
	var cmd tea.Cmd
	d.inputs[d.index], cmd = d.inputs[d.index].Update(msg)
	return cmd
}

func (d *dialog) submit() tea.Cmd {
	d.err = ""
	if d.kind == dialogProduce {
		value := d.inputs[1].Value()
		if value == "" {
			d.err = "value is required"
			return nil
		}
		params := domain.ProduceParams{
			Topic: d.topic,
			Key:   []byte(d.inputs[0].Value()),
			Value: []byte(value),
		}
		if d.inputs[0].Value() == "" {
			params.Key = nil
		}
		return func() tea.Msg { return submitProduceMsg{Params: params} }
	}

	name := strings.TrimSpace(d.inputs[0].Value())
	partitions, partitionsErr := strconv.Atoi(d.inputs[1].Value())
	replication, replicationErr := strconv.Atoi(d.inputs[2].Value())
	if name == "" || partitionsErr != nil || replicationErr != nil || partitions < 1 || replication < 1 {
		d.err = "name is required; partitions and replication must be positive integers"
		return nil
	}
	params := domain.CreateTopicParams{
		Name: name, Partitions: int32(partitions), ReplicationFactor: int16(replication),
	}
	return func() tea.Msg { return submitCreateTopicMsg{Params: params} }
}

func (d *dialog) view(styles Styles) string {
	title := "Create topic"
	if d.kind == dialogProduce {
		title = "Produce → " + d.topic
	}
	var body strings.Builder
	body.WriteString(styles.Header.Render(title))
	body.WriteString("\n\n")
	for i, input := range d.inputs {
		label := input.Placeholder
		if i == d.index {
			label = "● " + label
		} else {
			label = "  " + label
		}
		body.WriteString(styles.Muted.Render(label))
		body.WriteString("\n")
		body.WriteString(input.View())
		body.WriteString("\n\n")
	}
	if d.err != "" {
		body.WriteString(styles.Error.Render(d.err))
		body.WriteString("\n")
	}
	body.WriteString(styles.Muted.Render("[Tab] next  [Enter] submit  [Esc] cancel"))
	return styles.Modal.Width(64).Render(body.String())
}

func (d *dialog) String() string {
	return fmt.Sprintf("dialog(%d)", d.kind)
}
