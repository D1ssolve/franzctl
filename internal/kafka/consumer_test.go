package kafka

import (
	"strings"
	"testing"

	"github.com/twmb/franz-go/pkg/kgo"
)

func TestConsumerOptionsAreAcceptedByFranzGo(t *testing.T) {
	t.Parallel()

	partition := int32(2)
	tests := []struct {
		name   string
		config ConsumerConfig
	}{
		{
			name: "topic snapshot",
			config: ConsumerConfig{
				Topic:       "events",
				StartOffset: kgo.NewOffset().AtStart(),
			},
		},
		{
			name: "direct partition",
			config: ConsumerConfig{
				Topic:       "events",
				Partition:   &partition,
				StartOffset: kgo.NewOffset().At(42),
			},
		},
		{
			name: "consumer group without commits",
			config: ConsumerConfig{
				Topic:       "events",
				Group:       "inspectors",
				StartOffset: kgo.NewOffset().AtEnd(),
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			options, err := ConsumerOptions(test.config)
			if err != nil {
				t.Fatalf("ConsumerOptions() error = %v", err)
			}
			client, err := kgo.NewClient(append(
				[]kgo.Opt{kgo.SeedBrokers("localhost:9092")},
				options...,
			)...)
			if err != nil {
				t.Fatalf("kgo.NewClient() rejected options: %v", err)
			}
			client.Close()
		})
	}
}

func TestConsumerOptionsRejectInvalidModes(t *testing.T) {
	t.Parallel()

	partition := int32(0)
	negativePartition := int32(-1)
	tests := []struct {
		name      string
		config    ConsumerConfig
		wantError string
	}{
		{
			name:      "missing topic",
			config:    ConsumerConfig{},
			wantError: "topic must not be empty",
		},
		{
			name: "group with direct partition",
			config: ConsumerConfig{
				Topic:     "events",
				Partition: &partition,
				Group:     "inspectors",
			},
			wantError: "consumer group cannot be combined",
		},
		{
			name: "negative partition",
			config: ConsumerConfig{
				Topic:     "events",
				Partition: &negativePartition,
			},
			wantError: "partition must not be negative",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := ConsumerOptions(test.config)
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("ConsumerOptions() error = %v, want containing %q", err, test.wantError)
			}
		})
	}
}
