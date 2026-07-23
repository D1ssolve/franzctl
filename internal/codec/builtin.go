package codec

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

type transform struct {
	name   string
	encode func([]byte) ([]byte, error)
	decode func([]byte) ([]byte, error)
}

func (t transform) Name() string { return t.name }

func (t transform) Encode(_ context.Context, value []byte) ([]byte, error) {
	return t.encode(value)
}

func (t transform) Decode(_ context.Context, value []byte) ([]byte, error) {
	return t.decode(value)
}

func newStage(name string) (Stage, error) {
	identity := func(value []byte) ([]byte, error) { return value, nil }
	switch name {
	case "bytes", "string":
		return transform{name: name, encode: identity, decode: identity}, nil
	case "json":
		compact := func(value []byte) ([]byte, error) {
			var dst bytes.Buffer
			if err := json.Compact(&dst, value); err != nil {
				return nil, err
			}
			return dst.Bytes(), nil
		}
		return transform{name: name, encode: compact, decode: compact}, nil
	case "base64":
		return transform{
			name: name,
			encode: func(value []byte) ([]byte, error) {
				output := make([]byte, base64.StdEncoding.DecodedLen(len(value)))
				n, err := base64.StdEncoding.Decode(output, bytes.TrimSpace(value))
				return output[:n], err
			},
			decode: func(value []byte) ([]byte, error) {
				output := make([]byte, base64.StdEncoding.EncodedLen(len(value)))
				base64.StdEncoding.Encode(output, value)
				return output, nil
			},
		}, nil
	case "hex":
		return transform{
			name: name,
			encode: func(value []byte) ([]byte, error) {
				output := make([]byte, hex.DecodedLen(len(value)))
				n, err := hex.Decode(output, bytes.TrimSpace(value))
				return output[:n], err
			},
			decode: func(value []byte) ([]byte, error) {
				output := make([]byte, hex.EncodedLen(len(value)))
				hex.Encode(output, value)
				return output, nil
			},
		}, nil
	case "int64":
		return integerStage(name, true), nil
	case "uint64":
		return integerStage(name, false), nil
	case "float64":
		return transform{
			name: name,
			encode: func(value []byte) ([]byte, error) {
				n, err := strconv.ParseFloat(strings.TrimSpace(string(value)), 64)
				if err != nil {
					return nil, err
				}
				output := make([]byte, 8)
				binary.BigEndian.PutUint64(output, math.Float64bits(n))
				return output, nil
			},
			decode: func(value []byte) ([]byte, error) {
				if len(value) != 8 {
					return nil, fmt.Errorf("float64 requires 8 bytes, got %d", len(value))
				}
				n := math.Float64frombits(binary.BigEndian.Uint64(value))
				return []byte(strconv.FormatFloat(n, 'g', -1, 64)), nil
			},
		}, nil
	case "bool":
		return transform{
			name: name,
			encode: func(value []byte) ([]byte, error) {
				n, err := strconv.ParseBool(strings.TrimSpace(string(value)))
				if err != nil {
					return nil, err
				}
				if n {
					return []byte{1}, nil
				}
				return []byte{0}, nil
			},
			decode: func(value []byte) ([]byte, error) {
				if len(value) != 1 || value[0] > 1 {
					return nil, errors.New("bool must be exactly one byte: 0 or 1")
				}
				return []byte(strconv.FormatBool(value[0] == 1)), nil
			},
		}, nil
	case "gzip":
		return transform{name: name, encode: gzipEncode, decode: gzipDecode}, nil
	default:
		if path, ok := strings.CutPrefix(name, "exec:"); ok {
			if strings.TrimSpace(path) == "" {
				return nil, errors.New("exec codec requires an executable path")
			}
			return external{path: path}, nil
		}
		return nil, fmt.Errorf("unknown codec %q", name)
	}
}

func integerStage(name string, signed bool) Stage {
	return transform{
		name: name,
		encode: func(value []byte) ([]byte, error) {
			output := make([]byte, 8)
			if signed {
				n, err := strconv.ParseInt(strings.TrimSpace(string(value)), 10, 64)
				if err != nil {
					return nil, err
				}
				binary.BigEndian.PutUint64(output, uint64(n))
			} else {
				n, err := strconv.ParseUint(strings.TrimSpace(string(value)), 10, 64)
				if err != nil {
					return nil, err
				}
				binary.BigEndian.PutUint64(output, n)
			}
			return output, nil
		},
		decode: func(value []byte) ([]byte, error) {
			if len(value) != 8 {
				return nil, fmt.Errorf("%s requires 8 bytes, got %d", name, len(value))
			}
			n := binary.BigEndian.Uint64(value)
			if signed {
				return []byte(strconv.FormatInt(int64(n), 10)), nil
			}
			return []byte(strconv.FormatUint(n, 10)), nil
		},
	}
}

func gzipEncode(value []byte) ([]byte, error) {
	var output bytes.Buffer
	writer := gzip.NewWriter(&output)
	if _, err := writer.Write(value); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func gzipDecode(value []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(value))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}
