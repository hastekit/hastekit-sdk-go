package embeddings

import (
	"errors"

	"github.com/bytedance/sonic"
)

type Response struct {
	Object string          `json:"object"`
	Model  string          `json:"model"`
	Usage  *Usage          `json:"usage"`
	Data   []EmbeddingData `json:"data"`
}

type EmbeddingData struct {
	Object    string             `json:"object"`
	Index     int                `json:"index"`
	Embedding EmbeddingDataUnion `json:"embedding"`
}

type EmbeddingDataUnion struct {
	OfFloat  []float64 `json:",omitempty"`
	OfBase64 *string   `json:",omitempty"`
}

func (u *EmbeddingDataUnion) UnmarshalJSON(data []byte) error {
	var flt []float64
	if err := sonic.Unmarshal(data, &flt); err == nil {
		u.OfFloat = flt
		return nil
	}

	var str string
	if err := sonic.Unmarshal(data, &str); err == nil {
		u.OfBase64 = &str
		return nil
	}

	return errors.New("invalid embedding data union")
}

func (u *EmbeddingDataUnion) MarshalJSON() ([]byte, error) {
	if u.OfFloat != nil {
		return sonic.Marshal(u.OfFloat)
	}

	if u.OfBase64 != nil {
		return sonic.Marshal(u.OfBase64)
	}

	return nil, errors.New("invalid embedding data union")
}

type Usage struct {
	PromptTokens int64 `json:"prompt_tokens"`
	TotalTokens  int64 `json:"total_tokens"`
}
