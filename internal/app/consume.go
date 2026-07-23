package app

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/D1ssolve/franzctl/internal/codec"
	"github.com/D1ssolve/franzctl/internal/config"
	recordjson "github.com/D1ssolve/franzctl/internal/record"
)

type consumedHeader struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

type consumedRecord struct {
	Topic     string           `json:"topic"`
	Partition int32            `json:"partition"`
	Offset    int64            `json:"offset"`
	Timestamp time.Time        `json:"timestamp"`
	Key       any              `json:"key"`
	Value     any              `json:"value"`
	Headers   []consumedHeader `json:"headers,omitempty"`
}

func runConsume(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("consume", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var clientConfig config.Client
	clientConfig.Bind(fs)

	var topic, group, start, keyCodecSpec, valueCodecSpec, outputFormat string
	var keyPattern, valuePattern string
	var partition, maxRecords int
	var follow bool
	var pollTimeout time.Duration
	fs.StringVar(&topic, "topic", "", "topic name (required)")
	fs.IntVar(&partition, "partition", -1, "partition to read; -1 reads all partitions")
	fs.StringVar(&group, "group", "", "consumer group; offsets are not committed")
	fs.StringVar(&start, "from", "end", "starting position: beginning, end, or offset:N (partition required)")
	fs.IntVar(&maxRecords, "max", 0, "maximum records; 0 means unlimited")
	fs.BoolVar(&follow, "follow", true, "wait for new records")
	fs.DurationVar(&pollTimeout, "poll-timeout", time.Second, "wait time per broker poll")
	fs.StringVar(&keyCodecSpec, "key-codec", "bytes", "comma-separated key decoder pipeline")
	fs.StringVar(&valueCodecSpec, "value-codec", "bytes", "comma-separated value decoder pipeline")
	fs.StringVar(&keyPattern, "key-regex", "", "include records whose decoded key matches")
	fs.StringVar(&valuePattern, "value-regex", "", "include records whose decoded value matches")
	fs.StringVar(&outputFormat, "output", "jsonl", "output format: jsonl, pretty, raw")
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
	if partition < -1 || maxRecords < 0 || pollTimeout <= 0 {
		fmt.Fprintln(stderr, "--partition, --max, or --poll-timeout is invalid")
		return 2
	}
	if group != "" && partition >= 0 {
		fmt.Fprintln(stderr, "--group cannot be combined with explicit --partition")
		return 2
	}
	if outputFormat != "jsonl" && outputFormat != "pretty" && outputFormat != "raw" {
		fmt.Fprintln(stderr, "--output must be jsonl, pretty, or raw")
		return 2
	}

	startOffset, err := parseOffset(start)
	if err != nil {
		fmt.Fprintln(stderr, "--from:", err)
		return 2
	}
	if startOffset.Kind == offsetAbsolute && partition < 0 {
		fmt.Fprintln(stderr, "offset:N requires --partition")
		return 2
	}
	keyCodec, err := codec.Parse(keyCodecSpec)
	if err != nil {
		fmt.Fprintln(stderr, "--key-codec:", err)
		return 2
	}
	valueCodec, err := codec.Parse(valueCodecSpec)
	if err != nil {
		fmt.Fprintln(stderr, "--value-codec:", err)
		return 2
	}
	keyRE, err := compilePattern(keyPattern)
	if err != nil {
		fmt.Fprintln(stderr, "--key-regex:", err)
		return 2
	}
	valueRE, err := compilePattern(valuePattern)
	if err != nil {
		fmt.Fprintln(stderr, "--value-regex:", err)
		return 2
	}

	offset := startOffset.Kafka()
	consumerOptions := []kgo.Opt{kgo.DisableAutoCommit()}
	if partition >= 0 {
		consumerOptions = append(consumerOptions, kgo.ConsumePartitions(map[string]map[int32]kgo.Offset{
			topic: {int32(partition): offset},
		}))
	} else {
		consumerOptions = append(consumerOptions, kgo.ConsumeTopics(topic), kgo.ConsumeResetOffset(offset))
	}
	if group != "" {
		consumerOptions = append(consumerOptions, kgo.ConsumerGroup(group))
	}
	opts, err := clientConfig.Options(consumerOptions...)
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

	ctx, cancel := signalContext()
	defer cancel()
	encoder := json.NewEncoder(stdout)
	encoder.SetEscapeHTML(false)
	if outputFormat == "pretty" {
		encoder.SetIndent("", "  ")
	}

	written := 0
	for maxRecords == 0 || written < maxRecords {
		pollCtx, pollCancel := context.WithTimeout(ctx, pollTimeout)
		fetches := client.PollRecords(pollCtx, 100)
		pollCancel()
		if errors.Is(ctx.Err(), context.Canceled) {
			return 0
		}
		if errs := fetches.Errors(); len(errs) > 0 {
			for _, fetchErr := range errs {
				fmt.Fprintf(stderr, "consume %s[%d]: %v\n", fetchErr.Topic, fetchErr.Partition, fetchErr.Err)
			}
			return 1
		}
		records := fetches.Records()
		if len(records) == 0 {
			if !follow {
				return 0
			}
			continue
		}
		for _, kafkaRecord := range records {
			key, err := keyCodec.Decode(ctx, kafkaRecord.Key)
			if err != nil {
				fmt.Fprintf(stderr, "decode key at %s[%d]@%d: %v\n", kafkaRecord.Topic, kafkaRecord.Partition, kafkaRecord.Offset, err)
				return 1
			}
			value, err := valueCodec.Decode(ctx, kafkaRecord.Value)
			if err != nil {
				fmt.Fprintf(stderr, "decode value at %s[%d]@%d: %v\n", kafkaRecord.Topic, kafkaRecord.Partition, kafkaRecord.Offset, err)
				return 1
			}
			if (keyRE != nil && !keyRE.Match(key)) || (valueRE != nil && !valueRE.Match(value)) {
				continue
			}
			if outputFormat == "raw" {
				if _, err := stdout.Write(append(value, '\n')); err != nil {
					fmt.Fprintln(stderr, "write output:", err)
					return 1
				}
			} else {
				output := consumedRecord{
					Topic: kafkaRecord.Topic, Partition: kafkaRecord.Partition,
					Offset: kafkaRecord.Offset, Timestamp: kafkaRecord.Timestamp,
					Key: recordjson.Value(key), Value: recordjson.Value(value),
				}
				for _, header := range kafkaRecord.Headers {
					output.Headers = append(output.Headers, consumedHeader{
						Key: header.Key, Value: recordjson.Value(header.Value),
					})
				}
				if err := encoder.Encode(output); err != nil {
					fmt.Fprintln(stderr, "write output:", err)
					return 1
				}
			}
			written++
			if maxRecords > 0 && written >= maxRecords {
				return 0
			}
		}
	}
	return 0
}

func compilePattern(pattern string) (*regexp.Regexp, error) {
	if pattern == "" {
		return nil, nil
	}
	return regexp.Compile(pattern)
}

type offsetKind int

const (
	offsetBeginning offsetKind = iota
	offsetEnd
	offsetAbsolute
)

type parsedOffset struct {
	Kind  offsetKind
	Value int64
}

func parseOffset(value string) (parsedOffset, error) {
	switch value {
	case "beginning":
		return parsedOffset{Kind: offsetBeginning}, nil
	case "end":
		return parsedOffset{Kind: offsetEnd}, nil
	}
	if raw, ok := strings.CutPrefix(value, "offset:"); ok {
		offset, err := strconv.ParseInt(raw, 10, 64)
		if err == nil && offset >= 0 {
			return parsedOffset{Kind: offsetAbsolute, Value: offset}, nil
		}
	}
	return parsedOffset{}, fmt.Errorf("expected beginning, end, or offset:N; got %q", value)
}

func (o parsedOffset) Kafka() kgo.Offset {
	switch o.Kind {
	case offsetBeginning:
		return kgo.NewOffset().AtStart()
	case offsetAbsolute:
		return kgo.NewOffset().At(o.Value)
	default:
		return kgo.NewOffset().AtEnd()
	}
}
