package parser

import (
	"regexp"
	"strconv"
)

// CrossFeatureRef represents a reference to one or more features found in a
// "Predecessors:" field, e.g. "Feature 005" or "Features 003-005 (label)".
// FeatureNums holds the raw integer feature numbers; callers use
// fmt.Sprintf("%03d", n) to derive the zero-padded filename segment.
type CrossFeatureRef struct {
	FeatureNums []int  // feature number(s); always at least one element
	Label       string // optional label from parentheses; empty when absent
}

// crossFeatureSingularRe matches "Feature NNN" or "Feature NNN (label)" (case-insensitive).
// Capture groups: (1) feature number, (2) label (empty string when absent).
var crossFeatureSingularRe = regexp.MustCompile(`(?i)^feature\s+(\d+)(?:\s+\(([^)]*)\))?$`)

// crossFeatureRangeRe matches "Features NNN-MMM" or "Features NNN-MMM (label)" (case-insensitive).
// Capture groups: (1) from number, (2) to number, (3) label (empty string when absent).
var crossFeatureRangeRe = regexp.MustCompile(`(?i)^features\s+(\d+)-(\d+)(?:\s+\(([^)]*)\))?$`)

// parseCrossFeatureToken attempts to parse a single predecessor token as a
// cross-feature reference. Returns the CrossFeatureRef and true on success, or
// a zero value and false when the token is not a cross-feature reference.
func parseCrossFeatureToken(token string) (CrossFeatureRef, bool) {
	// Try singular: "Feature NNN" or "Feature NNN (label)"
	if m := crossFeatureSingularRe.FindStringSubmatch(token); m != nil {
		n, err := strconv.Atoi(m[1])
		if err != nil {
			return CrossFeatureRef{}, false
		}
		return CrossFeatureRef{
			FeatureNums: []int{n},
			Label:       m[2],
		}, true
	}

	// Try range: "Features NNN-MMM" or "Features NNN-MMM (label)"
	if m := crossFeatureRangeRe.FindStringSubmatch(token); m != nil {
		from, err1 := strconv.Atoi(m[1])
		to, err2 := strconv.Atoi(m[2])
		if err1 != nil || err2 != nil {
			return CrossFeatureRef{}, false
		}
		nums := make([]int, 0, to-from+1)
		for i := from; i <= to; i++ {
			nums = append(nums, i)
		}
		return CrossFeatureRef{
			FeatureNums: nums,
			Label:       m[3],
		}, true
	}

	return CrossFeatureRef{}, false
}
