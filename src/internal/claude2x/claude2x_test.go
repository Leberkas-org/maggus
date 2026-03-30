package claude2x

import (
	"testing"
	"time"
)

// makeWeekday returns a Monday at the given hour:minute:second UTC.
func makeWeekday(h, m, s int) time.Time {
	// 2026-03-02 is a Monday
	return time.Date(2026, 3, 2, h, m, s, 0, time.UTC)
}

// makeWeekend returns a Saturday at the given hour:minute:second UTC.
func makeWeekend(h, m, s int) time.Time {
	// 2026-03-07 is a Saturday
	return time.Date(2026, 3, 7, h, m, s, 0, time.UTC)
}

func TestComputeFromTime_Weekday_Normal(t *testing.T) {
	// Outside 13:00–19:00 → normal (not nerfed)
	for _, tc := range []struct{ h, m, s int }{
		{0, 0, 0}, {6, 0, 0}, {12, 59, 59}, {19, 0, 0}, {23, 59, 59},
	} {
		s := computeFromTime(makeWeekday(tc.h, tc.m, tc.s))
		if s.IsNerfed {
			t.Errorf("at %02d:%02d:%02d expected normal (IsNerfed=false), got IsNerfed=true", tc.h, tc.m, tc.s)
		}
	}
}

func TestComputeFromTime_Weekday_NerfedHours(t *testing.T) {
	// 13:00–19:00 weekday → nerfed
	for _, tc := range []struct{ h, m, s int }{
		{13, 0, 0}, {16, 0, 0}, {18, 59, 59},
	} {
		st := computeFromTime(makeWeekday(tc.h, tc.m, tc.s))
		if !st.IsNerfed {
			t.Errorf("at %02d:%02d:%02d expected IsNerfed=true", tc.h, tc.m, tc.s)
		}
		if st.TwoXWindowExpiresInSeconds <= 0 {
			t.Errorf("at %02d:%02d:%02d expected positive remaining seconds, got %d", tc.h, tc.m, tc.s, st.TwoXWindowExpiresInSeconds)
		}
	}
}

func TestComputeFromTime_Weekday_Boundaries(t *testing.T) {
	// Exactly at 13:00 → nerfed, 6h remaining
	s := computeFromTime(makeWeekday(13, 0, 0))
	if !s.IsNerfed {
		t.Error("at 13:00:00 expected nerfed")
	}
	if s.TwoXWindowExpiresInSeconds != 6*3600 {
		t.Errorf("at 13:00:00 expected 6h remaining, got %d", s.TwoXWindowExpiresInSeconds)
	}

	// Exactly at 19:00 → normal
	s = computeFromTime(makeWeekday(19, 0, 0))
	if s.IsNerfed {
		t.Error("at 19:00:00 expected normal (not nerfed)")
	}

	// Exactly at 01:00 → normal
	s = computeFromTime(makeWeekday(1, 0, 0))
	if s.IsNerfed {
		t.Error("at 01:00:00 expected normal (not nerfed)")
	}
}

func TestComputeFromTime_Weekend(t *testing.T) {
	// Weekends are never nerfed, even during 13:00–19:00
	for _, tc := range []struct{ h, m, s int }{
		{0, 0, 0}, {13, 0, 0}, {16, 0, 0}, {23, 59, 59},
	} {
		st := computeFromTime(makeWeekend(tc.h, tc.m, tc.s))
		if st.IsNerfed {
			t.Errorf("Saturday %02d:%02d expected IsNerfed=false", tc.h, tc.m)
		}
	}

	// Sunday
	sunday := time.Date(2026, 3, 8, 15, 0, 0, 0, time.UTC)
	if computeFromTime(sunday).IsNerfed {
		t.Error("Sunday expected IsNerfed=false")
	}
}

func TestFetchStatus_TestOverride_Active(t *testing.T) {
	SetTestCache(true, 3600)
	t.Cleanup(ResetTestCache)

	s := FetchStatus()
	if !s.IsNerfed {
		t.Error("expected IsNerfed=true from test override")
	}
	if s.TwoXWindowExpiresInSeconds < 3595 {
		t.Errorf("expected ~3600 remaining, got %d", s.TwoXWindowExpiresInSeconds)
	}
}

func TestFetchStatus_TestOverride_Expired(t *testing.T) {
	SetTestCache(true, 0)
	t.Cleanup(ResetTestCache)

	s := FetchStatus()
	if s.IsNerfed {
		t.Error("expected IsNerfed=false when remainingSeconds=0")
	}
}

func TestFetchStatus_TestOverride_Reset(t *testing.T) {
	SetTestCache(true, 3600)
	ResetTestCache()

	// After reset, should use real time logic without panic
	_ = FetchStatus()
}

func TestFormatRemaining_HoursMinSec(t *testing.T) {
	got := formatRemaining(64484)
	if got != "17h 54m 44s" {
		t.Errorf("expected '17h 54m 44s', got %q", got)
	}
}

func TestFormatRemaining_HoursMinSec2(t *testing.T) {
	got := formatRemaining(3723)
	if got != "1h 2m 3s" {
		t.Errorf("expected '1h 2m 3s', got %q", got)
	}
}

func TestFormatRemaining_MinSec(t *testing.T) {
	got := formatRemaining(125)
	if got != "2m 5s" {
		t.Errorf("expected '2m 5s', got %q", got)
	}
}

func TestFormatRemaining_SecOnly(t *testing.T) {
	got := formatRemaining(45)
	if got != "45s" {
		t.Errorf("expected '45s', got %q", got)
	}
}

func TestFormatRemaining_Zero(t *testing.T) {
	got := formatRemaining(0)
	if got != "0s" {
		t.Errorf("expected '0s', got %q", got)
	}
}
