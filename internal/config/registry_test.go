package config

import "testing"

func sampleRegistry() Registry {
	return NewRegistry([]Model{
		{
			ID:                  "gemini-3-pro-image",
			Label:               "Gemini 3 Pro Image",
			Enabled:             false,
			AllowedSizes:        []string{"1024x1024"},
			AllowedAspectRatios: []string{"1:1"},
			DefaultSize:         "1024x1024",
		},
		{
			ID:                  "gpt-image-2",
			Label:               "GPT Image 2",
			Enabled:             true,
			AllowedSizes:        []string{"1024x1024", "1536x1024"},
			AllowedAspectRatios: []string{"1:1", "3:2"},
			DefaultSize:         "1024x1024",
		},
	})
}

func TestRegistry_Lookup_found_enabled(t *testing.T) {
	r := sampleRegistry()
	m, ok, err := r.Lookup("gpt-image-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected ok=true for enabled model")
	}
	if m.ID != "gpt-image-2" {
		t.Fatalf("unexpected model id: %s", m.ID)
	}
	if !m.Enabled {
		t.Fatalf("expected Enabled=true")
	}
}

func TestRegistry_Lookup_found_disabled(t *testing.T) {
	r := sampleRegistry()
	m, ok, err := r.Lookup("gemini-3-pro-image")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Disabled models must still report ok=true so legacy node reads resolve.
	if !ok {
		t.Fatalf("expected ok=true for disabled-but-registered model")
	}
	if m.ID != "gemini-3-pro-image" {
		t.Fatalf("unexpected model id: %s", m.ID)
	}
	if m.Enabled {
		t.Fatalf("expected Enabled=false")
	}
}

func TestRegistry_Lookup_missing(t *testing.T) {
	r := sampleRegistry()
	m, ok, err := r.Lookup("does-not-exist")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatalf("expected ok=false for unknown model")
	}
	if m.ID != "" {
		t.Fatalf("expected zero Model, got id=%s", m.ID)
	}
}

func TestRegistry_EnabledList(t *testing.T) {
	r := sampleRegistry()
	list := r.EnabledList()
	if len(list) != 1 {
		t.Fatalf("expected 1 enabled model, got %d", len(list))
	}
	if list[0].ID != "gpt-image-2" {
		t.Fatalf("expected gpt-image-2, got %s", list[0].ID)
	}
}

func TestRegistry_Default(t *testing.T) {
	r := sampleRegistry()
	d := r.Default()
	if d.ID != "gemini-3-pro-image" {
		t.Fatalf("expected default to be first registry entry, got %s", d.ID)
	}
}

func TestRegistry_IsValid_happy(t *testing.T) {
	r := sampleRegistry()
	code, ok := r.IsValid("gpt-image-2", "1024x1024", "1:1")
	if !ok || code != "" {
		t.Fatalf("expected ok=true, code=\"\"; got ok=%v code=%q", ok, code)
	}
}

func TestRegistry_IsValid_unknown_model(t *testing.T) {
	r := sampleRegistry()
	code, ok := r.IsValid("unknown-model", "1024x1024", "1:1")
	if ok {
		t.Fatalf("expected ok=false for unknown model")
	}
	if code != "invalid" {
		t.Fatalf("expected code=invalid, got %q", code)
	}
}

func TestRegistry_IsValid_disabled_model(t *testing.T) {
	r := sampleRegistry()
	code, ok := r.IsValid("gemini-3-pro-image", "1024x1024", "1:1")
	if ok {
		t.Fatalf("expected ok=false for disabled model")
	}
	if code != "model_disabled" {
		t.Fatalf("expected code=model_disabled, got %q", code)
	}
}

func TestRegistry_IsValid_bad_size(t *testing.T) {
	r := sampleRegistry()
	code, ok := r.IsValid("gpt-image-2", "9999x9999", "1:1")
	if ok {
		t.Fatalf("expected ok=false for bad size")
	}
	if code != "invalid" {
		t.Fatalf("expected code=invalid, got %q", code)
	}
}

func TestRegistry_IsValid_bad_ratio(t *testing.T) {
	r := sampleRegistry()
	code, ok := r.IsValid("gpt-image-2", "1024x1024", "16:9")
	if ok {
		t.Fatalf("expected ok=false for bad ratio")
	}
	if code != "invalid" {
		t.Fatalf("expected code=invalid, got %q", code)
	}
}

func TestRegistry_IsValid_empty_size_and_ratio_skipped(t *testing.T) {
	// Empty size / ratio should be treated as "not provided" and accepted.
	r := sampleRegistry()
	code, ok := r.IsValid("gpt-image-2", "", "")
	if !ok || code != "" {
		t.Fatalf("expected ok=true when size/ratio omitted; got ok=%v code=%q", ok, code)
	}
}
