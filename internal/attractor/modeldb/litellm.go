package modeldb

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
)

// LiteLLMCatalog is a metadata-only model catalog loaded from the LiteLLM-maintained JSON file
// (commonly named model_prices_and_context_window.json).
type LiteLLMCatalog struct {
	Path   string
	SHA256 string
	Models map[string]LiteLLMModelEntry
}

type LiteLLMModelEntry struct {
	LiteLLMProvider string `json:"litellm_provider"`
	Mode            string `json:"mode"`

	MaxInputTokens  any `json:"max_input_tokens"`  // may be number or string in upstream
	MaxOutputTokens any `json:"max_output_tokens"` // may be number or string in upstream
	MaxTokens       any `json:"max_tokens"`        // legacy

	InputCostPerToken           *float64 `json:"input_cost_per_token"`
	OutputCostPerToken          *float64 `json:"output_cost_per_token"`
	OutputCostPerReasoningToken *float64 `json:"output_cost_per_reasoning_token"`

	DeprecationDate string `json:"deprecation_date"`
}

func LoadLiteLLMCatalog(path string) (*LiteLLMCatalog, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(b)
	sha := hex.EncodeToString(sum[:])

	var models map[string]LiteLLMModelEntry
	if err := json.Unmarshal(b, &models); err != nil {
		return nil, err
	}
	if len(models) == 0 {
		return nil, fmt.Errorf("litellm catalog is empty: %s", path)
	}
	return &LiteLLMCatalog{
		Path:   path,
		SHA256: sha,
		Models: models,
	}, nil
}
