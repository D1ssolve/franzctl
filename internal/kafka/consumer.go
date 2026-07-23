package kafka

import (
	"fmt"
	"strings"

	"github.com/twmb/franz-go/pkg/kgo"
)

// ConsumerConfig describes one read-only consumption mode.
//
// franzctl never commits offsets. Direct topic and partition consumers do not
// need group commit options at all; group consumers explicitly disable commits.
// Keeping that invariant here prevents callers from constructing option sets
// that franz-go rejects at startup.
type ConsumerConfig struct {
	Topic       string
	Partition   *int32
	Group       string
	StartOffset kgo.Offset
}

func ConsumerOptions(cfg ConsumerConfig) ([]kgo.Opt, error) {
	topic := strings.TrimSpace(cfg.Topic)
	if topic == "" {
		return nil, fmt.Errorf("topic must not be empty")
	}

	group := strings.TrimSpace(cfg.Group)
	if cfg.Partition != nil {
		if *cfg.Partition < 0 {
			return nil, fmt.Errorf("partition must not be negative")
		}
		if group != "" {
			return nil, fmt.Errorf("consumer group cannot be combined with an explicit partition")
		}
		return []kgo.Opt{
			kgo.ConsumePartitions(map[string]map[int32]kgo.Offset{
				topic: {*cfg.Partition: cfg.StartOffset},
			}),
		}, nil
	}

	options := []kgo.Opt{
		kgo.ConsumeTopics(topic),
		kgo.ConsumeResetOffset(cfg.StartOffset),
	}
	if group != "" {
		options = append(options,
			kgo.ConsumerGroup(group),
			kgo.DisableAutoCommit(),
		)
	}
	return options, nil
}
