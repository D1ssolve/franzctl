package tui

import "github.com/D1ssolve/franzctl/internal/domain"

type topicsLoadedMsg struct {
	Topics []domain.Topic
	Err    error
}

type recordsLoadedMsg struct {
	Topic   string
	Records []domain.Record
	Err     error
}

type operationDoneMsg struct {
	Operation string
	Record    *domain.Record
	Err       error
}
