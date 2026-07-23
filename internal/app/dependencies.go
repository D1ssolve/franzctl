package app

import (
	"github.com/D1ssolve/franzctl/internal/config"
	"github.com/D1ssolve/franzctl/internal/kafka"
)

type Dependencies struct {
	Kafka kafka.Manager
}

func BuildDependencies(cfg config.Client) Dependencies {
	return Dependencies{Kafka: kafka.New(cfg)}
}
