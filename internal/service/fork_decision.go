package service

// NodeParams contains parameters relevant to fork decision logic.
type NodeParams struct {
	Prompt            string
	NegativePrompt    *string
	CfgScale          *float64
	AspectRatio       *string
	Seed              *int64
	Steps             *int
	ReferenceImageIDs []string
}

// ShouldFork returns true when any relevant parameter differs.
func ShouldFork(current, next NodeParams) bool {
	if current.Prompt != next.Prompt {
		return true
	}
	if !equalStringPtr(current.NegativePrompt, next.NegativePrompt) {
		return true
	}
	if !equalFloat64Ptr(current.CfgScale, next.CfgScale) {
		return true
	}
	if !equalStringPtr(current.AspectRatio, next.AspectRatio) {
		return true
	}
	if !equalInt64Ptr(current.Seed, next.Seed) {
		return true
	}
	if !equalIntPtr(current.Steps, next.Steps) {
		return true
	}
	if !equalStringSlice(current.ReferenceImageIDs, next.ReferenceImageIDs) {
		return true
	}
	return false
}

func equalStringSlice(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalStringPtr(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func equalFloat64Ptr(a, b *float64) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func equalInt64Ptr(a, b *int64) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func equalIntPtr(a, b *int) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
