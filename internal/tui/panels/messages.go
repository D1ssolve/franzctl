package panels

import "github.com/D1ssolve/franzctl/internal/domain"

type TopicSelectedMsg struct {
	Topic string
}

type RecordSelectedMsg struct {
	Record *domain.Record
}

type FocusTopicsMsg struct{}
type FocusRecordsMsg struct{}
type OpenCreateTopicMsg struct{}

type OpenProduceMsg struct {
	Topic string
}
