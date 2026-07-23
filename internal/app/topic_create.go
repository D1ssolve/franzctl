package app

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
	"github.com/D1ssolve/franzctl/internal/config"
)

type topicCreateResult struct {
	Topic             string `json:"topic"`
	Partitions        int32  `json:"partitions"`
	ReplicationFactor int16  `json:"replication_factor"`
	ValidatedOnly     bool   `json:"validated_only"`
}

func runTopicCreate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("topic create", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var clientConfig config.Client
	clientConfig.Bind(fs)

	var topic string
	var partitions, replication int
	var topicConfigs config.KeyValues
	var validateOnly bool
	fs.StringVar(&topic, "topic", "", "topic name (required)")
	fs.IntVar(&partitions, "partitions", 1, "number of partitions; -1 uses the broker default")
	fs.IntVar(&replication, "replication-factor", 1, "replication factor; -1 uses the broker default")
	fs.Var(&topicConfigs, "config", "topic configuration key=value; repeatable")
	fs.BoolVar(&validateOnly, "validate-only", false, "validate without creating the topic")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if topic == "" {
		fmt.Fprintln(stderr, "--topic is required")
		return 2
	}
	if partitions < -1 || partitions == 0 || replication < -1 || replication == 0 {
		fmt.Fprintln(stderr, "--partitions and --replication-factor must be -1 or positive")
		return 2
	}
	if partitions > int(^uint32(0)>>1) || replication > int(^uint16(0)>>1) {
		fmt.Fprintln(stderr, "partition or replication value is too large")
		return 2
	}

	opts, err := clientConfig.Options()
	if err != nil {
		fmt.Fprintln(stderr, "configuration:", err)
		return 2
	}
	client, err := kgo.NewClient(opts...)
	if err != nil {
		fmt.Fprintln(stderr, "create Kafka client:", err)
		return 1
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), clientConfig.RequestTimeout)
	defer cancel()

	request := kmsg.NewPtrCreateTopicsRequest()
	request.TimeoutMillis = int32(clientConfig.RequestTimeout / time.Millisecond)
	request.ValidateOnly = validateOnly
	requestTopic := kmsg.NewCreateTopicsRequestTopic()
	requestTopic.Topic = topic
	requestTopic.NumPartitions = int32(partitions)
	requestTopic.ReplicationFactor = int16(replication)
	for key, value := range topicConfigs {
		requestConfig := kmsg.NewCreateTopicsRequestTopicConfig()
		requestConfig.Name = key
		requestConfig.Value = kmsg.StringPtr(value)
		requestTopic.Configs = append(requestTopic.Configs, requestConfig)
	}
	request.Topics = append(request.Topics, requestTopic)

	response, err := request.RequestWith(ctx, client)
	if err != nil {
		fmt.Fprintln(stderr, "create topic request:", err)
		return 1
	}
	if len(response.Topics) != 1 {
		fmt.Fprintf(stderr, "unexpected response: wanted one topic, got %d\n", len(response.Topics))
		return 1
	}
	result := response.Topics[0]
	if err := kerr.ErrorForCode(result.ErrorCode); err != nil {
		if result.ErrorMessage != nil {
			fmt.Fprintf(stderr, "create topic: %v: %s\n", err, *result.ErrorMessage)
		} else {
			fmt.Fprintln(stderr, "create topic:", err)
		}
		return 1
	}

	output := topicCreateResult{
		Topic:             result.Topic,
		Partitions:        result.NumPartitions,
		ReplicationFactor: result.ReplicationFactor,
		ValidatedOnly:     validateOnly,
	}
	if output.Partitions == 0 {
		output.Partitions = int32(partitions)
	}
	if output.ReplicationFactor == 0 {
		output.ReplicationFactor = int16(replication)
	}
	if err := json.NewEncoder(stdout).Encode(output); err != nil {
		fmt.Fprintln(stderr, "write output:", err)
		return 1
	}
	return 0
}
