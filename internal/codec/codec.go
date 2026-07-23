package codec

import (
	"context"
	"fmt"
	"strings"
)

type Stage interface {
	Name() string
	Encode(context.Context, []byte) ([]byte, error)
	Decode(context.Context, []byte) ([]byte, error)
}

type Pipeline struct {
	stages []Stage
}

func Parse(spec string) (Pipeline, error) {
	if strings.TrimSpace(spec) == "" {
		spec = "bytes"
	}
	names := strings.Split(spec, ",")
	stages := make([]Stage, 0, len(names))
	for _, name := range names {
		stage, err := newStage(strings.TrimSpace(name))
		if err != nil {
			return Pipeline{}, err
		}
		stages = append(stages, stage)
	}
	return Pipeline{stages: stages}, nil
}

func (p Pipeline) Encode(ctx context.Context, input []byte) ([]byte, error) {
	var err error
	output := append([]byte(nil), input...)
	for _, stage := range p.stages {
		output, err = stage.Encode(ctx, output)
		if err != nil {
			return nil, fmt.Errorf("encode with %s: %w", stage.Name(), err)
		}
	}
	return output, nil
}

func (p Pipeline) Decode(ctx context.Context, input []byte) ([]byte, error) {
	var err error
	output := append([]byte(nil), input...)
	for i := len(p.stages) - 1; i >= 0; i-- {
		stage := p.stages[i]
		output, err = stage.Decode(ctx, output)
		if err != nil {
			return nil, fmt.Errorf("decode with %s: %w", stage.Name(), err)
		}
	}
	return output, nil
}

func Builtins() []string {
	return []string{
		"bytes", "string", "json", "base64", "hex",
		"int64", "uint64", "float64", "bool", "gzip",
		"exec:/absolute/path/to/codec",
	}
}
