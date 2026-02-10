package service

import "testing"

func TestForkDecision(t *testing.T) {
	base := NodeParams{
		Prompt:            "prompt",
		NegativePrompt:    stringPtr("negative"),
		CfgScale:          float64Ptr(7.5),
		AspectRatio:       stringPtr("1:1"),
		Seed:              int64Ptr(42),
		Steps:             intPtr(30),
		ReferenceImageIDs: []string{"img-1"},
	}

	tests := []struct {
		name    string
		current NodeParams
		next    NodeParams
		want    bool
	}{
		{
			name:    "identical params no fork",
			current: base,
			next:    base,
			want:    false,
		},
		{
			name:    "prompt changed",
			current: base,
			next:    withPrompt(base, "prompt v2"),
			want:    true,
		},
		{
			name: "aspect ratio nil vs non-nil",
			current: NodeParams{
				Prompt:            base.Prompt,
				NegativePrompt:    base.NegativePrompt,
				CfgScale:          base.CfgScale,
				AspectRatio:       nil,
				Seed:              base.Seed,
				Steps:             base.Steps,
				ReferenceImageIDs: base.ReferenceImageIDs,
			},
			next: base,
			want: true,
		},
		{
			name:    "aspect ratio value changed",
			current: base,
			next:    withAspectRatio(base, "16:9"),
			want:    true,
		},
		{
			name:    "reference image ids changed",
			current: base,
			next:    withReferenceImageIDs(base, []string{"img-2"}),
			want:    true,
		},
		{
			name:    "negative prompt changed",
			current: base,
			next:    withNegativePrompt(base, "negative v2"),
			want:    true,
		},
		{
			name:    "cfg scale changed",
			current: base,
			next:    withCfgScale(base, 8.0),
			want:    true,
		},
		{
			name:    "seed changed",
			current: base,
			next:    withSeed(base, 43),
			want:    true,
		},
		{
			name:    "steps changed",
			current: base,
			next:    withSteps(base, 31),
			want:    true,
		},
	}

	for _, tt := range tests {
		if got := ShouldFork(tt.current, tt.next); got != tt.want {
			t.Fatalf("%s: ShouldFork() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func stringPtr(v string) *string {
	return &v
}

func float64Ptr(v float64) *float64 {
	return &v
}

func int64Ptr(v int64) *int64 {
	return &v
}

func intPtr(v int) *int {
	return &v
}

func withPrompt(base NodeParams, value string) NodeParams {
	clone := base
	clone.Prompt = value
	return clone
}

func withNegativePrompt(base NodeParams, value string) NodeParams {
	clone := base
	clone.NegativePrompt = stringPtr(value)
	return clone
}

func withCfgScale(base NodeParams, value float64) NodeParams {
	clone := base
	clone.CfgScale = float64Ptr(value)
	return clone
}

func withAspectRatio(base NodeParams, value string) NodeParams {
	clone := base
	clone.AspectRatio = stringPtr(value)
	return clone
}

func withSeed(base NodeParams, value int64) NodeParams {
	clone := base
	clone.Seed = int64Ptr(value)
	return clone
}

func withSteps(base NodeParams, value int) NodeParams {
	clone := base
	clone.Steps = intPtr(value)
	return clone
}

func withReferenceImageIDs(base NodeParams, value []string) NodeParams {
	clone := base
	clone.ReferenceImageIDs = value
	return clone
}
