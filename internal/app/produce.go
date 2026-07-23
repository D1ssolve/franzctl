package app

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/D1ssolve/franzctl/internal/codec"
	"github.com/D1ssolve/franzctl/internal/config"
)

type optionalString struct {
	value string
	set   bool
}

func (o *optionalString) String() string { return o.value }
func (o *optionalString) Set(value string) error {
	o.value, o.set = value, true
	return nil
}

type producedRecord struct {
	Topic     string `json:"topic"`
	Partition int32  `json:"partition"`
	Offset    int64  `json:"offset"`
}

func runProduce(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("produce", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var clientConfig config.Client
	clientConfig.Bind(fs)

	var topic, keyCodecSpec, valueCodecSpec, inputMode string
	var partition int
	var key, value optionalString
	var headers config.KeyValues
	fs.StringVar(&topic, "topic", "", "topic name (required)")
	fs.Var(&key, "key", "message key; omitted means a null key")
	fs.Var(&value, "value", "single message value; when omitted, read stdin")
	fs.IntVar(&partition, "partition", -1, "target partition; -1 uses the Kafka partitioner")
	fs.Var(&headers, "header", "Kafka header key=value; repeatable")
	fs.StringVar(&keyCodecSpec, "key-codec", "string", "comma-separated key encoder pipeline")
	fs.StringVar(&valueCodecSpec, "value-codec", "string", "comma-separated value encoder pipeline")
	fs.StringVar(&inputMode, "input", "lines", "stdin mode: lines or raw")
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
	if partition < -1 {
		fmt.Fprintln(stderr, "--partition must be -1 or non-negative")
		return 2
	}
	if inputMode != "lines" && inputMode != "raw" {
		fmt.Fprintln(stderr, "--input must be lines or raw")
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

	producerOptions := []kgo.Opt{}
	if partition >= 0 {
		producerOptions = append(producerOptions, kgo.RecordPartitioner(kgo.ManualPartitioner()))
	}
	opts, err := clientConfig.Options(producerOptions...)
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
	var encodedKey []byte
	if key.set {
		encodedKey, err = keyCodec.Encode(ctx, []byte(key.value))
		if err != nil {
			fmt.Fprintln(stderr, "encode key:", err)
			return 1
		}
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetEscapeHTML(false)

	send := func(input []byte) error {
		encodedValue, err := valueCodec.Encode(ctx, input)
		if err != nil {
			return fmt.Errorf("encode value: %w", err)
		}
		kafkaRecord := &kgo.Record{Topic: topic, Key: encodedKey, Value: encodedValue}
		if partition >= 0 {
			kafkaRecord.Partition = int32(partition)
		}
		for headerKey, headerValue := range headers {
			kafkaRecord.Headers = append(kafkaRecord.Headers, kgo.RecordHeader{
				Key: headerKey, Value: []byte(headerValue),
			})
		}
		result := client.ProduceSync(ctx, kafkaRecord)
		if err := result.FirstErr(); err != nil {
			return err
		}
		ack := result[0].Record
		return encoder.Encode(producedRecord{
			Topic: ack.Topic, Partition: ack.Partition, Offset: ack.Offset,
		})
	}

	if value.set {
		if err := send([]byte(value.value)); err != nil {
			fmt.Fprintln(stderr, "produce:", err)
			return 1
		}
		return 0
	}
	if inputMode == "raw" {
		input, err := io.ReadAll(stdin)
		if err != nil {
			fmt.Fprintln(stderr, "read stdin:", err)
			return 1
		}
		if err := send(input); err != nil {
			fmt.Fprintln(stderr, "produce:", err)
			return 1
		}
		return 0
	}

	scanner := bufio.NewScanner(stdin)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		if err := send([]byte(strings.TrimSuffix(scanner.Text(), "\r"))); err != nil {
			fmt.Fprintln(stderr, "produce:", err)
			return 1
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(stderr, "read stdin:", err)
		return 1
	}
	return 0
}
