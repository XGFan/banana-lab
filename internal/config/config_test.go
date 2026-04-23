package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// minimalModelsYAML is a valid `models:` block with one enabled default model.
// Tests that care about the registry contents add more entries or override.
const minimalModelsYAML = `models:
  - id: test-default
    label: Test Default
    enabled: true
    allowed_sizes: ["1024x1024"]
    allowed_aspect_ratios: ["1:1"]
    default_size: "1024x1024"
`

func withTempDir(t *testing.T, fn func(dir string)) {
	t.Helper()
	dir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})
	fn(dir)
}

func writeConfigYAML(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(body), 0644); err != nil {
		t.Fatalf("write config.yaml failed: %v", err)
	}
}

func TestRequiredConfig(t *testing.T) {
	withTempDir(t, func(dir string) {
		requiredEnvValues := map[string]string{
			"PROXY_BASE_URL":          "https://proxy.example.com",
			"PROXY_AUTH_HEADER_VALUE": "token-123",
		}

		setOptionalEmpty := func(t *testing.T) {
			t.Helper()
			optionalKeys := []string{
				"PROXY_AUTH_HEADER_NAME",
				"SERVER_PORT",
				"MAX_CONCURRENT",
				"LIBRARY_PAGE_SIZE",
				"GENERATE_TIMEOUT",
				"STORAGE_ROOT",
				"DB_PATH",
			}
			for _, key := range optionalKeys {
				t.Setenv(key, "")
			}
		}

		setRequired := func(t *testing.T) {
			t.Helper()
			for key, value := range requiredEnvValues {
				t.Setenv(key, value)
			}
		}

		// Every "defaults" scenario needs a models: block because config.yaml
		// is the sole source of truth for the registry.
		writeConfigYAML(t, dir, minimalModelsYAML)

		for _, missingKey := range []string{
			"PROXY_BASE_URL",
			"PROXY_AUTH_HEADER_VALUE",
		} {
			missingKey := missingKey
			t.Run("missing "+missingKey, func(t *testing.T) {
				setRequired(t)
				setOptionalEmpty(t)
				t.Setenv(missingKey, "")

				_, err := LoadFromEnv()
				if err == nil {
					t.Fatalf("expected error when %s missing", missingKey)
				}
			})
		}

		t.Run("defaults", func(t *testing.T) {
			setRequired(t)
			setOptionalEmpty(t)

			cfg, err := LoadFromEnv()
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			if cfg.ProxyBaseURL != requiredEnvValues["PROXY_BASE_URL"] {
				t.Fatalf("unexpected ProxyBaseURL: %s", cfg.ProxyBaseURL)
			}
			if cfg.ProxyAuthHeaderValue != requiredEnvValues["PROXY_AUTH_HEADER_VALUE"] {
				t.Fatalf("unexpected ProxyAuthHeaderValue: %s", cfg.ProxyAuthHeaderValue)
			}
			if len(cfg.Models) != 1 || cfg.Models[0].ID != "test-default" {
				t.Fatalf("unexpected Models: %+v", cfg.Models)
			}
			if cfg.ServerPort != 8080 {
				t.Fatalf("unexpected ServerPort: %d", cfg.ServerPort)
			}
			if cfg.ProxyAuthHeaderName != "Authorization" {
				t.Fatalf("unexpected ProxyAuthHeaderName: %s", cfg.ProxyAuthHeaderName)
			}
			if cfg.MaxConcurrent != 1 {
				t.Fatalf("unexpected MaxConcurrent: %d", cfg.MaxConcurrent)
			}
			if cfg.LibraryPageSize != 20 {
				t.Fatalf("unexpected LibraryPageSize: %d", cfg.LibraryPageSize)
			}
			if cfg.GenerateTimeout != 120*time.Second {
				t.Fatalf("unexpected GenerateTimeout: %s", cfg.GenerateTimeout)
			}
		})
	})
}

func TestLoadFromFile(t *testing.T) {
	withTempDir(t, func(dir string) {
		config := []byte(`proxy_base_url: https://file.example.com
proxy_auth_header_value: file-token
proxy_auth_header_name: X-File
server_port: 9090
max_concurrent: 3
library_page_size: 50
generate_timeout: 45s
storage_root: ./from-file
db_path: ./from-file/db.sqlite
models:
  - id: file-default
    label: File Default
    enabled: true
    allowed_sizes: ["1024x1024"]
    allowed_aspect_ratios: ["1:1"]
    default_size: "1024x1024"
  - id: file-secondary
    label: File Secondary
    enabled: false
    allowed_sizes: ["1536x1024"]
    allowed_aspect_ratios: ["3:2"]
    default_size: "1536x1024"
`)
		if err := os.WriteFile(filepath.Join(dir, "config.yaml"), config, 0644); err != nil {
			t.Fatalf("write config.yaml failed: %v", err)
		}
		// Ensure env does not override file values.
		t.Setenv("PROXY_BASE_URL", "")
		t.Setenv("PROXY_AUTH_HEADER_VALUE", "")
		t.Setenv("SERVER_PORT", "")
		t.Setenv("MAX_CONCURRENT", "")
		t.Setenv("LIBRARY_PAGE_SIZE", "")
		t.Setenv("GENERATE_TIMEOUT", "")
		t.Setenv("PROXY_AUTH_HEADER_NAME", "")
		t.Setenv("STORAGE_ROOT", "")
		t.Setenv("DB_PATH", "")

		cfg, err := LoadFromEnv()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if cfg.ProxyBaseURL != "https://file.example.com" {
			t.Fatalf("unexpected ProxyBaseURL: %s", cfg.ProxyBaseURL)
		}
		if cfg.ProxyAuthHeaderValue != "file-token" {
			t.Fatalf("unexpected ProxyAuthHeaderValue: %s", cfg.ProxyAuthHeaderValue)
		}
		if len(cfg.Models) != 2 {
			t.Fatalf("expected 2 models, got %d", len(cfg.Models))
		}
		if cfg.Models[0].ID != "file-default" || !cfg.Models[0].Enabled {
			t.Fatalf("unexpected default model: %+v", cfg.Models[0])
		}
		if cfg.Models[1].ID != "file-secondary" || cfg.Models[1].Enabled {
			t.Fatalf("unexpected secondary model: %+v", cfg.Models[1])
		}
		if cfg.ProxyAuthHeaderName != "X-File" {
			t.Fatalf("unexpected ProxyAuthHeaderName: %s", cfg.ProxyAuthHeaderName)
		}
		if cfg.ServerPort != 9090 {
			t.Fatalf("unexpected ServerPort: %d", cfg.ServerPort)
		}
		if cfg.MaxConcurrent != 3 {
			t.Fatalf("unexpected MaxConcurrent: %d", cfg.MaxConcurrent)
		}
		if cfg.LibraryPageSize != 50 {
			t.Fatalf("unexpected LibraryPageSize: %d", cfg.LibraryPageSize)
		}
		if cfg.GenerateTimeout != 45*time.Second {
			t.Fatalf("unexpected GenerateTimeout: %s", cfg.GenerateTimeout)
		}
		if cfg.StorageRoot != "./from-file" {
			t.Fatalf("unexpected StorageRoot: %s", cfg.StorageRoot)
		}
		if cfg.DBPath != "./from-file/db.sqlite" {
			t.Fatalf("unexpected DBPath: %s", cfg.DBPath)
		}
	})
}

func TestEnvOverridesFile(t *testing.T) {
	withTempDir(t, func(dir string) {
		config := []byte(`proxy_base_url: https://file.example.com
proxy_auth_header_value: file-token
server_port: 9090
models:
  - id: file-default
    label: File Default
    enabled: true
    allowed_sizes: ["1024x1024"]
    allowed_aspect_ratios: ["1:1"]
    default_size: "1024x1024"
`)
		if err := os.WriteFile(filepath.Join(dir, "config.yaml"), config, 0644); err != nil {
			t.Fatalf("write config.yaml failed: %v", err)
		}
		t.Setenv("PROXY_BASE_URL", "https://env.example.com")
		t.Setenv("SERVER_PORT", "7070")

		cfg, err := LoadFromEnv()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if cfg.ProxyBaseURL != "https://env.example.com" {
			t.Fatalf("expected env override, got %s", cfg.ProxyBaseURL)
		}
		if cfg.ServerPort != 7070 {
			t.Fatalf("expected env override, got %d", cfg.ServerPort)
		}
	})
}

func TestLoadFromEnv_empty_models_registry_rejected(t *testing.T) {
	withTempDir(t, func(dir string) {
		writeConfigYAML(t, dir, `proxy_base_url: https://x.example.com
proxy_auth_header_value: token
`)
		t.Setenv("PROXY_BASE_URL", "")
		t.Setenv("PROXY_AUTH_HEADER_VALUE", "")

		_, err := LoadFromEnv()
		if err == nil {
			t.Fatalf("expected error for empty models registry")
		}
	})
}

func TestLoadFromEnv_disabled_only_registry_rejected(t *testing.T) {
	withTempDir(t, func(dir string) {
		writeConfigYAML(t, dir, `proxy_base_url: https://x.example.com
proxy_auth_header_value: token
models:
  - id: only-disabled
    label: Only Disabled
    enabled: false
    allowed_sizes: ["1024x1024"]
    allowed_aspect_ratios: ["1:1"]
    default_size: "1024x1024"
`)
		t.Setenv("PROXY_BASE_URL", "")
		t.Setenv("PROXY_AUTH_HEADER_VALUE", "")

		_, err := LoadFromEnv()
		if err == nil {
			t.Fatalf("expected error when every model is disabled")
		}
	})
}

func TestLoadFromEnv_first_entry_disabled_rejected(t *testing.T) {
	withTempDir(t, func(dir string) {
		writeConfigYAML(t, dir, `proxy_base_url: https://x.example.com
proxy_auth_header_value: token
models:
  - id: disabled-default
    label: Disabled Default
    enabled: false
    allowed_sizes: ["1024x1024"]
    allowed_aspect_ratios: ["1:1"]
    default_size: "1024x1024"
  - id: enabled-secondary
    label: Enabled Secondary
    enabled: true
    allowed_sizes: ["1024x1024"]
    allowed_aspect_ratios: ["1:1"]
    default_size: "1024x1024"
`)
		t.Setenv("PROXY_BASE_URL", "")
		t.Setenv("PROXY_AUTH_HEADER_VALUE", "")

		_, err := LoadFromEnv()
		if err == nil {
			t.Fatalf("expected error when Models[0] is disabled")
		}
	})
}

func TestLoadFromEnv_duplicate_ids_rejected(t *testing.T) {
	withTempDir(t, func(dir string) {
		writeConfigYAML(t, dir, `proxy_base_url: https://x.example.com
proxy_auth_header_value: token
models:
  - id: dup-id
    label: First
    enabled: true
    allowed_sizes: ["1024x1024"]
    allowed_aspect_ratios: ["1:1"]
    default_size: "1024x1024"
  - id: dup-id
    label: Second
    enabled: true
    allowed_sizes: ["1024x1024"]
    allowed_aspect_ratios: ["1:1"]
    default_size: "1024x1024"
`)
		t.Setenv("PROXY_BASE_URL", "")
		t.Setenv("PROXY_AUTH_HEADER_VALUE", "")

		_, err := LoadFromEnv()
		if err == nil {
			t.Fatalf("expected error for duplicate model ids")
		}
	})
}
