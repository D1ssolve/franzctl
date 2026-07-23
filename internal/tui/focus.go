package tui

type FocusPanel int

const (
	FocusTopics FocusPanel = iota
	FocusRecords
)

func (f FocusPanel) String() string {
	switch f {
	case FocusRecords:
		return "records"
	default:
		return "topics"
	}
}
