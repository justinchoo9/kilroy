package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type BackendKind string

const (
	BackendAPI BackendKind = "api"
	BackendCLI BackendKind = "cli"
)

type RunConfigFile struct {
	Version int `json:"version" yaml:"version"`

	Repo struct {
		Path string `json:"path" yaml:"path"`
	} `json:"repo" yaml:"repo"`

	CXDB struct {
		BinaryAddr  string `json:"binary_addr" yaml:"binary_addr"`
		HTTPBaseURL string `json:"http_base_url" yaml:"http_base_url"`
	} `json:"cxdb" yaml:"cxdb"`

	LLM struct {
		Providers map[string]struct {
			Backend BackendKind `json:"backend" yaml:"backend"`
		} `json:"providers" yaml:"providers"`
	} `json:"llm" yaml:"llm"`

	ModelDB struct {
		LiteLLMCatalogPath           string `json:"litellm_catalog_path" yaml:"litellm_catalog_path"`
		LiteLLMCatalogUpdatePolicy   string `json:"litellm_catalog_update_policy" yaml:"litellm_catalog_update_policy"`
		LiteLLMCatalogURL            string `json:"litellm_catalog_url" yaml:"litellm_catalog_url"`
		LiteLLMCatalogFetchTimeoutMS int    `json:"litellm_catalog_fetch_timeout_ms" yaml:"litellm_catalog_fetch_timeout_ms"`
	} `json:"modeldb" yaml:"modeldb"`

	Git struct {
		RequireClean    bool   `json:"require_clean" yaml:"require_clean"`
		RunBranchPrefix string `json:"run_branch_prefix" yaml:"run_branch_prefix"`
		CommitPerNode   bool   `json:"commit_per_node" yaml:"commit_per_node"`
	} `json:"git" yaml:"git"`
}

func LoadRunConfigFile(path string) (*RunConfigFile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg RunConfigFile
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		if err := json.Unmarshal(b, &cfg); err != nil {
			return nil, err
		}
	default:
		if err := yaml.Unmarshal(b, &cfg); err != nil {
			return nil, err
		}
	}
	applyConfigDefaults(&cfg)
	if err := validateConfig(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func applyConfigDefaults(cfg *RunConfigFile) {
	if cfg == nil {
		return
	}
	if cfg.Version == 0 {
		cfg.Version = 1
	}
	if cfg.Git.RunBranchPrefix == "" {
		cfg.Git.RunBranchPrefix = "attractor/run"
	}
	// metaspec default.
	if !cfg.Git.CommitPerNode {
		cfg.Git.CommitPerNode = true
	}
	// metaspec default.
	if !cfg.Git.RequireClean {
		cfg.Git.RequireClean = true
	}
	if cfg.LLM.Providers == nil {
		cfg.LLM.Providers = map[string]struct {
			Backend BackendKind `json:"backend" yaml:"backend"`
		}{}
	}
	if strings.TrimSpace(cfg.ModelDB.LiteLLMCatalogUpdatePolicy) == "" {
		cfg.ModelDB.LiteLLMCatalogUpdatePolicy = "on_run_start"
	}
	if strings.TrimSpace(cfg.ModelDB.LiteLLMCatalogURL) == "" {
		cfg.ModelDB.LiteLLMCatalogURL = "https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json"
	}
	if cfg.ModelDB.LiteLLMCatalogFetchTimeoutMS == 0 {
		cfg.ModelDB.LiteLLMCatalogFetchTimeoutMS = 5000
	}
}

func validateConfig(cfg *RunConfigFile) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	if cfg.Version != 1 {
		return fmt.Errorf("unsupported config version: %d", cfg.Version)
	}
	if strings.TrimSpace(cfg.Repo.Path) == "" {
		return fmt.Errorf("repo.path is required")
	}
	if strings.TrimSpace(cfg.CXDB.BinaryAddr) == "" || strings.TrimSpace(cfg.CXDB.HTTPBaseURL) == "" {
		return fmt.Errorf("cxdb.binary_addr and cxdb.http_base_url are required in v1")
	}
	if strings.TrimSpace(cfg.ModelDB.LiteLLMCatalogPath) == "" {
		return fmt.Errorf("modeldb.litellm_catalog_path is required")
	}
	switch strings.ToLower(strings.TrimSpace(cfg.ModelDB.LiteLLMCatalogUpdatePolicy)) {
	case "pinned", "on_run_start":
		// ok
	default:
		return fmt.Errorf("invalid modeldb.litellm_catalog_update_policy: %q (want pinned|on_run_start)", cfg.ModelDB.LiteLLMCatalogUpdatePolicy)
	}
	if strings.ToLower(strings.TrimSpace(cfg.ModelDB.LiteLLMCatalogUpdatePolicy)) == "on_run_start" && strings.TrimSpace(cfg.ModelDB.LiteLLMCatalogURL) == "" {
		return fmt.Errorf("modeldb.litellm_catalog_url is required when update_policy=on_run_start")
	}
	for prov, pc := range cfg.LLM.Providers {
		switch normalizeProviderKey(prov) {
		case "openai", "anthropic", "google":
			// ok
		default:
			return fmt.Errorf("unsupported provider in config: %q", prov)
		}
		switch pc.Backend {
		case BackendAPI, BackendCLI:
		default:
			return fmt.Errorf("invalid backend for provider %q: %q (want api|cli)", prov, pc.Backend)
		}
	}
	return nil
}

func normalizeProviderKey(k string) string {
	k = strings.ToLower(strings.TrimSpace(k))
	switch k {
	case "gemini":
		return "google"
	default:
		return k
	}
}
