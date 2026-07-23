package kafka

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"

	"github.com/D1ssolve/franzctl/internal/config"
	"github.com/D1ssolve/franzctl/internal/domain"
)

type Client struct {
	cfg config.Client
}

func New(cfg config.Client) *Client {
	return &Client{cfg: cfg}
}

func (c *Client) newClient(extra ...kgo.Opt) (*kgo.Client, error) {
	opts, err := c.cfg.Options(extra...)
	if err != nil {
		return nil, err
	}
	return kgo.NewClient(opts...)
}

func (c *Client) ListTopics(ctx context.Context) ([]domain.Topic, error) {
	client, err := c.newClient()
	if err != nil {
		return nil, err
	}
	defer client.Close()

	request := kmsg.NewPtrMetadataRequest()
	response, err := request.RequestWith(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("metadata request: %w", err)
	}

	topics := make([]domain.Topic, 0, len(response.Topics))
	for _, item := range response.Topics {
		if item.Topic == nil {
			continue
		}
		if topicErr := kerr.ErrorForCode(item.ErrorCode); topicErr != nil {
			return nil, fmt.Errorf("topic %q: %w", *item.Topic, topicErr)
		}
		replication := 0
		if len(item.Partitions) > 0 {
			replication = len(item.Partitions[0].Replicas)
		}
		topics = append(topics, domain.Topic{
			Name:              *item.Topic,
			Partitions:        len(item.Partitions),
			ReplicationFactor: replication,
			Internal:          item.IsInternal,
		})
	}
	sort.Slice(topics, func(i, j int) bool { return topics[i].Name < topics[j].Name })
	return topics, nil
}

func (c *Client) ReadRecords(ctx context.Context, topic string, limit int) ([]domain.Record, error) {
	if limit <= 0 {
		limit = 100
	}
	client, err := c.newClient(
		kgo.ConsumeTopics(topic),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
		kgo.DisableAutoCommit(),
	)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	fetches := client.PollRecords(ctx, limit)
	if errs := fetches.Errors(); len(errs) > 0 {
		return nil, errs[0].Err
	}

	records := make([]domain.Record, 0, len(fetches.Records()))
	for _, record := range fetches.Records() {
		item := domain.Record{
			Topic: record.Topic, Partition: record.Partition, Offset: record.Offset,
			Timestamp: record.Timestamp, Key: append([]byte(nil), record.Key...),
			Value: append([]byte(nil), record.Value...),
		}
		for _, header := range record.Headers {
			item.Headers = append(item.Headers, domain.Header{
				Key: header.Key, Value: append([]byte(nil), header.Value...),
			})
		}
		records = append(records, item)
	}
	return records, nil
}

func (c *Client) CreateTopic(ctx context.Context, params domain.CreateTopicParams) error {
	client, err := c.newClient()
	if err != nil {
		return err
	}
	defer client.Close()

	request := kmsg.NewPtrCreateTopicsRequest()
	request.TimeoutMillis = int32(c.cfg.RequestTimeout / time.Millisecond)
	topic := kmsg.NewCreateTopicsRequestTopic()
	topic.Topic = params.Name
	topic.NumPartitions = params.Partitions
	topic.ReplicationFactor = params.ReplicationFactor
	request.Topics = append(request.Topics, topic)

	response, err := request.RequestWith(ctx, client)
	if err != nil {
		return err
	}
	if len(response.Topics) != 1 {
		return fmt.Errorf("unexpected create topic response")
	}
	return kerr.ErrorForCode(response.Topics[0].ErrorCode)
}

func (c *Client) Produce(ctx context.Context, params domain.ProduceParams) (domain.Record, error) {
	client, err := c.newClient()
	if err != nil {
		return domain.Record{}, err
	}
	defer client.Close()

	result := client.ProduceSync(ctx, &kgo.Record{
		Topic: params.Topic,
		Key:   params.Key,
		Value: params.Value,
	})
	if err := result.FirstErr(); err != nil {
		return domain.Record{}, err
	}
	record := result[0].Record
	return domain.Record{
		Topic: record.Topic, Partition: record.Partition, Offset: record.Offset,
		Timestamp: record.Timestamp, Key: record.Key, Value: record.Value,
	}, nil
}
