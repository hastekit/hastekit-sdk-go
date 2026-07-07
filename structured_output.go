package sdk

import (
	"log/slog"

	json "github.com/bytedance/sonic"
	"github.com/invopop/jsonschema"
)

type OutputSchema = jsonschema.Schema

func NewOutputSchema(v any) map[string]any {
	buf, err := jsonschema.Reflect(v).MarshalJSON()
	if err != nil {
		slog.Warn("failed to reflect output schema: " + err.Error())
		return nil
	}

	ss := map[string]any{}
	err = json.Unmarshal(buf, &ss)
	if err != nil {
		slog.Warn("failed to unmarshal output schema: " + err.Error())
		return nil
	}

	return ss
}
