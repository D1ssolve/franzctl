package record

import (
	"encoding/base64"
	"encoding/json"
	"unicode/utf8"
)

type Binary struct {
	Encoding string `json:"encoding"`
	Data     string `json:"data"`
}

func Value(value []byte) any {
	if value == nil {
		return nil
	}
	var decoded any
	if json.Valid(value) && json.Unmarshal(value, &decoded) == nil {
		return decoded
	}
	if utf8.Valid(value) {
		return string(value)
	}
	return Binary{Encoding: "base64", Data: base64.StdEncoding.EncodeToString(value)}
}
