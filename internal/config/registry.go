package config

// Registry is a thin lookup wrapper over a slice of Models loaded from config.yaml.
// The first entry is treated as the default model.
type Registry struct {
	Models []Model
}

// NewRegistry wraps the supplied model slice. Callers should have already
// validated the slice via validateModels during config load.
func NewRegistry(models []Model) Registry {
	return Registry{Models: models}
}

// Lookup returns the model with the matching ID. The boolean indicates whether
// the model was found in the registry, regardless of whether it is currently
// enabled — this allows legacy node reads to resolve historical model IDs.
// The error return is reserved for future failure modes and is always nil today.
func (r Registry) Lookup(id string) (Model, bool, error) {
	for _, m := range r.Models {
		if m.ID == id {
			return m, true, nil
		}
	}
	return Model{}, false, nil
}

// EnabledList returns only the models with Enabled=true, preserving registry order.
func (r Registry) EnabledList() []Model {
	out := make([]Model, 0, len(r.Models))
	for _, m := range r.Models {
		if m.Enabled {
			out = append(out, m)
		}
	}
	return out
}

// Default returns the first model in the registry. Config validation guarantees
// at least one model exists and that Models[0].Enabled == true.
func (r Registry) Default() Model {
	if len(r.Models) == 0 {
		return Model{}
	}
	return r.Models[0]
}

// IsValid reports whether a (model id, size, aspect ratio) triple is acceptable
// for a generation request. The returned code is a machine-readable error code:
//   - ""               : valid
//   - "invalid"        : unknown model id, or size/ratio not in the model's allowed list
//   - "model_disabled" : model id is known but currently disabled
//
// Empty size or ratio strings are treated as "not provided" and skipped — callers
// are responsible for enforcing their own presence rules.
func (r Registry) IsValid(id, size, ratio string) (string, bool) {
	model, found, _ := r.Lookup(id)
	if !found {
		return "invalid", false
	}
	if !model.Enabled {
		return "model_disabled", false
	}
	if size != "" && !containsString(model.AllowedSizes, size) {
		return "invalid", false
	}
	if ratio != "" && !containsString(model.AllowedAspectRatios, ratio) {
		return "invalid", false
	}
	return "", true
}

func containsString(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}

// IsKnownSize reports whether the provided image size string is allowed by at
// least one model in the registry. Used by reroll validation when no explicit
// model override is present: the node's existing model_id is re-used by the
// service layer, but the API still rejects blatantly invalid size strings
// (e.g. "3K") so legacy reroll error semantics are preserved.
func (r Registry) IsKnownSize(size string) bool {
	if size == "" {
		return true
	}
	for _, m := range r.Models {
		if containsString(m.AllowedSizes, size) {
			return true
		}
	}
	return false
}
