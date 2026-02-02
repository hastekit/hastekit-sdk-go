package utils

import (
	"io"

	"github.com/bytedance/sonic"
)

func DecodeJSON(r io.Reader, v any) error {
	body, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	return sonic.Unmarshal(body, v)
}
