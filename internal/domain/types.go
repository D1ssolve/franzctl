package domain

import "time"

type Topic struct {
	Name              string
	Partitions        int
	ReplicationFactor int
	Internal          bool
}

type Header struct {
	Key   string
	Value []byte
}

type Record struct {
	Topic     string
	Partition int32
	Offset    int64
	Timestamp time.Time
	Key       []byte
	Value     []byte
	Headers   []Header
}

type CreateTopicParams struct {
	Name              string
	Partitions        int32
	ReplicationFactor int16
}

type ProduceParams struct {
	Topic string
	Key   []byte
	Value []byte
}
