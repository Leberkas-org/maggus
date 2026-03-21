package cmd

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leberkas-org/maggus/internal/claude2x"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

// assertTickScheduled checks that the returned tea.Cmd produces a claude2xTickMsg.
func assertTickScheduled(t *testing.T, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected a tick command to be scheduled, got nil")
	}
}

// assertNoTickScheduled checks that no tick command was returned.
func assertNoTickScheduled(t *testing.T, cmd tea.Cmd) {
	t.Helper()
	if cmd != nil {
		t.Fatal("expected no tick command, got non-nil")
	}
}

func TestMenuUpdate_Claude2xTickMsg_StillActive(t *testing.T) {
	// Seed the claude2x cache with an active 2x status.
	claude2x.SetTestCache(true, 3600)
	t.Cleanup(func() { claude2x.ResetTestCache() })

	m := menuModel{
		items: activeMenuItems(),
		is2x:  true,
	}

	model, cmd := m.Update(claude2xTickMsg{})
	mm := model.(menuModel)

	if !mm.is2x {
		t.Error("expected is2x to remain true")
	}
	if mm.twoXExpiresIn == "" {
		t.Error("expected twoXExpiresIn to be set")
	}
	assertTickScheduled(t, cmd)
}

func TestMenuUpdate_Claude2xTickMsg_Expired(t *testing.T) {
	// Seed the cache with an expired 2x status.
	claude2x.SetTestCache(true, 0)
	t.Cleanup(func() { claude2x.ResetTestCache() })

	m := menuModel{
		items:         activeMenuItems(),
		is2x:          true,
		twoXExpiresIn: "1s",
	}

	model, cmd := m.Update(claude2xTickMsg{})
	mm := model.(menuModel)

	if mm.is2x {
		t.Error("expected is2x to be false after expiry")
	}
	if mm.twoXExpiresIn != "" {
		t.Errorf("expected twoXExpiresIn to be empty, got %q", mm.twoXExpiresIn)
	}
	assertNoTickScheduled(t, cmd)
}

func TestMenuUpdate_Claude2xResultMsg_SchedulesTick(t *testing.T) {
	m := menuModel{items: activeMenuItems()}

	model, cmd := m.Update(claude2xResultMsg{status: claude2x.Status{
		Is2x:                true,
		TwoXWindowExpiresIn: "1h 0m 0s",
	}})
	mm := model.(menuModel)

	if !mm.is2x {
		t.Error("expected is2x to be true")
	}
	assertTickScheduled(t, cmd)
}

func TestMenuUpdate_Claude2xResultMsg_NoTickWhenNot2x(t *testing.T) {
	m := menuModel{items: activeMenuItems()}

	model, cmd := m.Update(claude2xResultMsg{status: claude2x.Status{Is2x: false}})
	mm := model.(menuModel)

	if mm.is2x {
		t.Error("expected is2x to be false")
	}
	assertNoTickScheduled(t, cmd)
}

func TestStatusUpdate_Claude2xTickMsg_StillActive(t *testing.T) {
	claude2x.SetTestCache(true, 3600)
	t.Cleanup(func() { claude2x.ResetTestCache() })

	m := statusModel{is2x: true}

	model, cmd := m.Update(claude2xTickMsg{})
	sm := model.(statusModel)

	if !sm.is2x {
		t.Error("expected is2x to remain true")
	}
	if sm.BorderColor != styles.ThemeColor(true) {
		t.Error("expected border color to match 2x theme")
	}
	assertTickScheduled(t, cmd)
}

func TestStatusUpdate_Claude2xTickMsg_Expired(t *testing.T) {
	claude2x.SetTestCache(true, 0)
	t.Cleanup(func() { claude2x.ResetTestCache() })

	m := statusModel{is2x: true}

	model, cmd := m.Update(claude2xTickMsg{})
	sm := model.(statusModel)

	if sm.is2x {
		t.Error("expected is2x to be false after expiry")
	}
	if sm.BorderColor != styles.ThemeColor(false) {
		t.Error("expected border color to reset to default")
	}
	assertNoTickScheduled(t, cmd)
}

func TestConfigUpdate_Claude2xTickMsg_StillActive(t *testing.T) {
	claude2x.SetTestCache(true, 3600)
	t.Cleanup(func() { claude2x.ResetTestCache() })

	m := configModel{is2x: true}

	model, cmd := m.Update(claude2xTickMsg{})
	cm := model.(configModel)

	if !cm.is2x {
		t.Error("expected is2x to remain true")
	}
	assertTickScheduled(t, cmd)
}

func TestConfigUpdate_Claude2xTickMsg_Expired(t *testing.T) {
	claude2x.SetTestCache(true, 0)
	t.Cleanup(func() { claude2x.ResetTestCache() })

	m := configModel{is2x: true}

	model, cmd := m.Update(claude2xTickMsg{})
	cm := model.(configModel)

	if cm.is2x {
		t.Error("expected is2x to be false after expiry")
	}
	assertNoTickScheduled(t, cmd)
}

func TestReposUpdate_Claude2xTickMsg_StillActive(t *testing.T) {
	claude2x.SetTestCache(true, 3600)
	t.Cleanup(func() { claude2x.ResetTestCache() })

	m := reposModel{is2x: true}

	model, cmd := m.Update(claude2xTickMsg{})
	rm := model.(reposModel)

	if !rm.is2x {
		t.Error("expected is2x to remain true")
	}
	assertTickScheduled(t, cmd)
}

func TestReposUpdate_Claude2xTickMsg_Expired(t *testing.T) {
	claude2x.SetTestCache(true, 0)
	t.Cleanup(func() { claude2x.ResetTestCache() })

	m := reposModel{is2x: true}

	model, cmd := m.Update(claude2xTickMsg{})
	rm := model.(reposModel)

	if rm.is2x {
		t.Error("expected is2x to be false after expiry")
	}
	assertNoTickScheduled(t, cmd)
}

func TestUpdateModelUpdate_Claude2xTickMsg_StillActive(t *testing.T) {
	claude2x.SetTestCache(true, 3600)
	t.Cleanup(func() { claude2x.ResetTestCache() })

	m := updateModel{is2x: true}

	model, cmd := m.Update(claude2xTickMsg{})
	um := model.(updateModel)

	if !um.is2x {
		t.Error("expected is2x to remain true")
	}
	assertTickScheduled(t, cmd)
}

func TestUpdateModelUpdate_Claude2xTickMsg_Expired(t *testing.T) {
	claude2x.SetTestCache(true, 0)
	t.Cleanup(func() { claude2x.ResetTestCache() })

	m := updateModel{is2x: true}

	model, cmd := m.Update(claude2xTickMsg{})
	um := model.(updateModel)

	if um.is2x {
		t.Error("expected is2x to be false after expiry")
	}
	assertNoTickScheduled(t, cmd)
}
