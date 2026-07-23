package kafka

import (
	"context"

	"github.com/D1ssolve/franzctl/internal/domain"
)

type Manager interface {
	ListTopics(context.Context) ([]domain.Topic, error)
	ReadRecords(context.Context, string, int) ([]domain.Record, error)
	CreateTopic(context.Context, domain.CreateTopicParams) error
	Produce(context.Context, domain.ProduceParams) (domain.Record, error)
}
