package codec

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
)

type external struct {
	path string
}

func (e external) Name() string { return "exec:" + e.path }

func (e external) Encode(ctx context.Context, value []byte) ([]byte, error) {
	return e.run(ctx, "encode", value)
}

func (e external) Decode(ctx context.Context, value []byte) ([]byte, error) {
	return e.run(ctx, "decode", value)
}

func (e external) run(ctx context.Context, mode string, value []byte) ([]byte, error) {
	command := exec.CommandContext(ctx, e.path)
	command.Env = append(os.Environ(), "FRANZCTL_CODEC_MODE="+mode)
	command.Stdin = bytes.NewReader(value)

	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		message := stderr.String()
		if len(message) > 1024 {
			message = message[:1024] + "…"
		}
		return nil, fmt.Errorf("%w: %s", err, message)
	}
	return stdout.Bytes(), nil
}
