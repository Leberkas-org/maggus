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
	claude2x.SetTestCache(true, 3600)
	t.Cleanup(func() { claude2x.ResetTestCache() })

	m := menuModel{
		items:    activeMenuItems(),
		isNerfed: true,
	}

	model, cmd := m.Update(claude2xTickMsg{})
	mm := model.(menuModel)

	if !mm.isNerfed {
		t.Error("expected isNerfed to remain true")
	}
	if mm.twoXExpiresIn == "" {
		t.Error("expected twoXExpiresIn to be set")
	}
	assertTickScheduled(t, cmd)
}

func TestMenuUpdate_Claude2xTickMsg_Expired(t *testing.T) {
	claude2x.SetTestCache(true, 0)
	t.Cleanup(func() { claude2x.ResetTestCache() })

	m := menuModel{
		items:         activeMenuItems(),
		isNerfed:      true,
		twoXExpiresIn: "1s",
	}

	model, cmd := m.Update(claude2xTickMsg{})
	mm := model.(menuModel)

	if mm.isNerfed {
		t.Error("expected isNerfed to be false after expiry")
	}
	if mm.twoXExpiresIn != "" {
		t.Errorf("expected twoXExpiresIn to be empty, got %q", mm.twoXExpiresIn)
	}
	assertNoTickScheduled(t, cmd)
}

func TestMenuUpdate_Claude2xResultMsg_SchedulesTick(t *testing.T) {
	m := menuModel{items: activeMenuItems()}

	model, cmd := m.Update(claude2xResultMsg{status: claude2x.Status{
		IsNerfed:            true,
		TwoXWindowExpiresIn: "1h 0m 0s",
	}})
	mm := model.(menuModel)

	if !mm.isNerfed {
		t.Error("expected isNerfed to be true")
	}
	assertTickScheduled(t, cmd)
}

func TestMenuUpdate_Claude2xResultMsg_NoTickWhenNormal(t *testing.T) {
	m := menuModel{items: activeMenuItems()}

	model, cmd := m.Update(claude2xResultMsg{status: claude2x.Status{IsNerfed: false}})
	mm := model.(menuModel)

	if mm.isNerfed {
		t.Error("expected isNerfed to be false")
	}
	assertNoTickScheduled(t, cmd)
}

// TestMenuUpdate_Claude2xResultMsg_AlreadyNerfed_NoDoubleTick verifies that when
// isNerfed was eagerly seeded in newMenuModel (so Init already started next2xTick),
// a subsequent claude2xResultMsg with the same nerfed=true state does NOT start a
// second concurrent tick chain.
func TestMenuUpdate_Claude2xResultMsg_AlreadyNerfed_NoDoubleTick(t *testing.T) {
	m := menuModel{
		items:    activeMenuItems(),
		isNerfed: true, // simulates state seeded eagerly in newMenuModel
	}

	model, cmd := m.Update(claude2xResultMsg{status: claude2x.Status{
		IsNerfed:            true,
		TwoXWindowExpiresIn: "1h 0m 0s",
	}})
	mm := model.(menuModel)

	if !mm.isNerfed {
		t.Error("expected isNerfed to remain true")
	}
	// When already nerfed (tick started from Init), claude2xResultMsg must NOT
	// schedule another tick to avoid duplicate concurrent tick chains.
	assertNoTickScheduled(t, cmd)
}

func TestStatusUpdate_Claude2xTickMsg_StillActive(t *testing.T) {
	claude2x.SetTestCache(true, 3600)
	t.Cleanup(func() { claude2x.ResetTestCache() })

	m := statusModel{isNerfed: true}

	model, cmd := m.Update(claude2xTickMsg{})
	sm := model.(statusModel)

	if !sm.isNerfed {
		t.Error("expected isNerfed to remain true")
	}
	if sm.BorderColor != styles.ThemeColor(true) {
		t.Error("expected border color to match nerfed theme")
	}
	assertTickScheduled(t, cmd)
}

func TestStatusUpdate_Claude2xTickMsg_Expired(t *testing.T) {
	claude2x.SetTestCache(true, 0)
	t.Cleanup(func() { claude2x.ResetTestCache() })

	m := statusModel{isNerfed: true}

	model, cmd := m.Update(claude2xTickMsg{})
	sm := model.(statusModel)

	if sm.isNerfed {
		t.Error("expected isNerfed to be false after expiry")
	}
	if sm.BorderColor != styles.ThemeColor(false) {
		t.Error("expected border color to reset to default")
	}
	assertNoTickScheduled(t, cmd)
}

func TestConfigUpdate_Claude2xTickMsg_StillActive(t *testing.T) {
	claude2x.SetTestCache(true, 3600)
	t.Cleanup(func() { claude2x.ResetTestCache() })

	m := configModel{isNerfed: true}

	model, cmd := m.Update(claude2xTickMsg{})
	cm := model.(configModel)

	if !cm.isNerfed {
		t.Error("expected isNerfed to remain true")
	}
	assertTickScheduled(t, cmd)
}

func TestConfigUpdate_Claude2xTickMsg_Expired(t *testing.T) {
	claude2x.SetTestCache(true, 0)
	t.Cleanup(func() { claude2x.ResetTestCache() })

	m := configModel{isNerfed: true}

	model, cmd := m.Update(claude2xTickMsg{})
	cm := model.(configModel)

	if cm.isNerfed {
		t.Error("expected isNerfed to be false after expiry")
	}
	assertNoTickScheduled(t, cmd)
}

func TestReposUpdate_Claude2xTickMsg_StillActive(t *testing.T) {
	claude2x.SetTestCache(true, 3600)
	t.Cleanup(func() { claude2x.ResetTestCache() })

	m := reposModel{isNerfed: true}

	model, cmd := m.Update(claude2xTickMsg{})
	rm := model.(reposModel)

	if !rm.isNerfed {
		t.Error("expected isNerfed to remain true")
	}
	assertTickScheduled(t, cmd)
}

func TestReposUpdate_Claude2xTickMsg_Expired(t *testing.T) {
	claude2x.SetTestCache(true, 0)
	t.Cleanup(func() { claude2x.ResetTestCache() })

	m := reposModel{isNerfed: true}

	model, cmd := m.Update(claude2xTickMsg{})
	rm := model.(reposModel)

	if rm.isNerfed {
		t.Error("expected isNerfed to be false after expiry")
	}
	assertNoTickScheduled(t, cmd)
}

func TestUpdateModelUpdate_Claude2xTickMsg_StillActive(t *testing.T) {
	claude2x.SetTestCache(true, 3600)
	t.Cleanup(func() { claude2x.ResetTestCache() })

	m := updateModel{isNerfed: true}

	model, cmd := m.Update(claude2xTickMsg{})
	um := model.(updateModel)

	if !um.isNerfed {
		t.Error("expected isNerfed to remain true")
	}
	assertTickScheduled(t, cmd)
}

func TestUpdateModelUpdate_Claude2xTickMsg_Expired(t *testing.T) {
	claude2x.SetTestCache(true, 0)
	t.Cleanup(func() { claude2x.ResetTestCache() })

	m := updateModel{isNerfed: true}

	model, cmd := m.Update(claude2xTickMsg{})
	um := model.(updateModel)

	if um.isNerfed {
		t.Error("expected isNerfed to be false after expiry")
	}
	assertNoTickScheduled(t, cmd)
}
