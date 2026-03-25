package cmd

import "testing"

func TestVerbForTask(t *testing.T) {
	tests := []struct {
		name       string
		sourceFile string
		want       string
	}{
		{
			name:       "feature file unix path",
			sourceFile: ".maggus/features/feature_001.md",
			want:       "Working",
		},
		{
			name:       "feature file windows path",
			sourceFile: `.maggus\features\feature_001.md`,
			want:       "Working",
		},
		{
			name:       "bug file unix path",
			sourceFile: ".maggus/bugs/bug_001.md",
			want:       "Fixing",
		},
		{
			name:       "bug file windows path",
			sourceFile: `.maggus\bugs\bug_001.md`,
			want:       "Fixing",
		},
		{
			name:       "absolute bug path unix",
			sourceFile: "/home/user/project/.maggus/bugs/bug_002.md",
			want:       "Fixing",
		},
		{
			name:       "absolute bug path windows",
			sourceFile: `C:\projects\app\.maggus\bugs\bug_002.md`,
			want:       "Fixing",
		},
		{
			name:       "empty string defaults to Working",
			sourceFile: "",
			want:       "Working",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := verbForTask(tt.sourceFile)
			if got != tt.want {
				t.Errorf("verbForTask(%q) = %q, want %q", tt.sourceFile, got, tt.want)
			}
		})
	}
}
