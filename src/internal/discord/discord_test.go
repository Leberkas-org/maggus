package discord

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"net"
	"testing"
	"time"
)

func TestFormatDetails(t *testing.T) {
	tests := []struct {
		name  string
		state PresenceState
		want  string
	}{
		{
			name:  "full state",
			state: PresenceState{TaskID: "TASK-003-002", TaskTitle: "Add login page", FeatureTitle: "User Auth"},
			want:  "User Auth \u2014 TASK-003-002: Add login page",
		},
		{
			name:  "no feature title",
			state: PresenceState{TaskID: "TASK-001-001", TaskTitle: "Setup"},
			want:  "TASK-001-001: Setup",
		},
		{
			name:  "no task ID",
			state: PresenceState{FeatureTitle: "User Auth"},
			want:  "User Auth",
		},
		{
			name:  "empty state",
			state: PresenceState{},
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatDetails(tt.state)
			if got != tt.want {
				t.Errorf("FormatDetails() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildActivity(t *testing.T) {
	start := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	state := PresenceState{
		TaskID:       "TASK-001-003",
		TaskTitle:    "Implement discord package",
		FeatureTitle: "Discord Rich Presence",
		StartTime:    start,
	}

	a := buildActivity(state)

	if a.State != "Running Maggus" {
		t.Errorf("State = %q, want %q", a.State, "Running Maggus")
	}
	if a.Assets == nil || a.Assets.LargeImage != AssetKeyLargeImage {
		t.Errorf("LargeImage = %v, want %q", a.Assets, AssetKeyLargeImage)
	}
	if a.Timestamps == nil || a.Timestamps.Start == nil {
		t.Fatal("expected non-nil timestamps with start")
	}
	if *a.Timestamps.Start != start.UnixMilli() {
		t.Errorf("Timestamps.Start = %d, want %d", *a.Timestamps.Start, start.UnixMilli())
	}
	wantDetails := "Discord Rich Presence \u2014 TASK-001-003: Implement discord package"
	if a.Details != wantDetails {
		t.Errorf("Details = %q, want %q", a.Details, wantDetails)
	}
}

func TestBuildActivityNoTimestamp(t *testing.T) {
	state := PresenceState{TaskID: "TASK-001", TaskTitle: "Test"}
	a := buildActivity(state)

	if a.Timestamps != nil {
		t.Errorf("expected nil timestamps for zero StartTime, got %+v", a.Timestamps)
	}
}

func TestWriteAndReadMessage(t *testing.T) {
	type testPayload struct {
		Cmd  string `json:"cmd"`
		Data string `json:"data"`
	}

	original := testPayload{Cmd: "TEST", Data: "hello"}
	var buf bytes.Buffer

	if err := writeMessage(&buf, opFrame, original); err != nil {
		t.Fatalf("writeMessage: %v", err)
	}

	opcode, raw, err := readMessage(&buf)
	if err != nil {
		t.Fatalf("readMessage: %v", err)
	}

	if opcode != opFrame {
		t.Errorf("opcode = %d, want %d", opcode, opFrame)
	}

	var got testPayload
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got != original {
		t.Errorf("payload = %+v, want %+v", got, original)
	}
}

func TestMessageFraming(t *testing.T) {
	payload := map[string]string{"key": "value"}
	var buf bytes.Buffer

	if err := writeMessage(&buf, opHandshake, payload); err != nil {
		t.Fatalf("writeMessage: %v", err)
	}

	data := buf.Bytes()
	if len(data) < 8 {
		t.Fatalf("message too short: %d bytes", len(data))
	}

	// Verify header.
	opcode := binary.LittleEndian.Uint32(data[0:4])
	length := binary.LittleEndian.Uint32(data[4:8])

	if opcode != opHandshake {
		t.Errorf("header opcode = %d, want %d", opcode, opHandshake)
	}
	if int(length) != len(data)-8 {
		t.Errorf("header length = %d, want %d", length, len(data)-8)
	}

	// Verify the JSON payload is valid.
	var parsed map[string]string
	if err := json.Unmarshal(data[8:], &parsed); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}
	if parsed["key"] != "value" {
		t.Errorf("payload key = %q, want %q", parsed["key"], "value")
	}
}

func TestBuildSetActivityPayload(t *testing.T) {
	state := PresenceState{
		TaskID:       "TASK-001-003",
		TaskTitle:    "Test task",
		FeatureTitle: "Feature",
		StartTime:    time.Now(),
	}

	p := buildSetActivityPayload(1234, state)

	if p.Cmd != "SET_ACTIVITY" {
		t.Errorf("Cmd = %q, want %q", p.Cmd, "SET_ACTIVITY")
	}
	if p.Args.PID != 1234 {
		t.Errorf("PID = %d, want %d", p.Args.PID, 1234)
	}
	if p.Args.Activity == nil {
		t.Fatal("expected non-nil Activity")
	}
	if p.Nonce == "" {
		t.Error("expected non-empty Nonce")
	}
}

func TestBuildClearActivityPayload(t *testing.T) {
	p := buildClearActivityPayload(5678)

	if p.Cmd != "SET_ACTIVITY" {
		t.Errorf("Cmd = %q, want %q", p.Cmd, "SET_ACTIVITY")
	}
	if p.Args.PID != 5678 {
		t.Errorf("PID = %d, want %d", p.Args.PID, 5678)
	}
	if p.Args.Activity != nil {
		t.Errorf("expected nil Activity for clear, got %+v", p.Args.Activity)
	}
}

func TestSetActivitySerializesToJSON(t *testing.T) {
	state := PresenceState{
		TaskID:       "TASK-001-003",
		TaskTitle:    "Implement discord",
		FeatureTitle: "Discord Presence",
		StartTime:    time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
	}

	p := buildSetActivityPayload(100, state)

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	// Verify it round-trips correctly.
	var decoded activityPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if decoded.Cmd != "SET_ACTIVITY" {
		t.Errorf("Cmd = %q, want SET_ACTIVITY", decoded.Cmd)
	}
	if decoded.Args.Activity == nil {
		t.Fatal("expected Activity in deserialized payload")
	}
	if decoded.Args.Activity.State != "Running Maggus" {
		t.Errorf("State = %q, want 'Running Maggus'", decoded.Args.Activity.State)
	}
	if decoded.Args.Activity.Assets == nil || decoded.Args.Activity.Assets.LargeImage != "maggus_logo" {
		t.Error("expected maggus_logo in assets")
	}
}

// fakeConn is a net.Conn backed by a bytes.Buffer for capturing writes.
// Reads return a canned IPC response so readMessage succeeds.
type fakeConn struct {
	written bytes.Buffer
	readBuf bytes.Buffer
}

func newFakeConn() *fakeConn {
	fc := &fakeConn{}
	// Pre-fill a valid IPC response so the readMessage in Close() succeeds.
	_ = writeMessage(&fc.readBuf, opFrame, map[string]string{"cmd": "SET_ACTIVITY"})
	return fc
}

func (f *fakeConn) Read(b []byte) (int, error)  { return f.readBuf.Read(b) }
func (f *fakeConn) Write(b []byte) (int, error)  { return f.written.Write(b) }
func (f *fakeConn) Close() error                 { return nil }
func (f *fakeConn) LocalAddr() net.Addr          { return nil }
func (f *fakeConn) RemoteAddr() net.Addr         { return nil }
func (f *fakeConn) SetDeadline(time.Time) error  { return nil }
func (f *fakeConn) SetReadDeadline(time.Time) error { return nil }
func (f *fakeConn) SetWriteDeadline(time.Time) error { return nil }

func TestPresenceCloseWritesClearAndOpClose(t *testing.T) {
	fc := newFakeConn()
	p := &Presence{
		conn:      fc,
		connected: true,
	}

	if err := p.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	// Parse the frames written to the connection.
	data := fc.written.Bytes()
	if len(data) < 8 {
		t.Fatalf("expected at least one frame, got %d bytes", len(data))
	}

	// First frame: opFrame with clear-activity payload.
	op1 := binary.LittleEndian.Uint32(data[0:4])
	len1 := binary.LittleEndian.Uint32(data[4:8])
	if op1 != opFrame {
		t.Errorf("first frame opcode = %d, want %d (opFrame)", op1, opFrame)
	}

	// Verify the clear-activity payload has nil activity.
	var payload1 activityPayload
	if err := json.Unmarshal(data[8:8+len1], &payload1); err != nil {
		t.Fatalf("unmarshal first frame: %v", err)
	}
	if payload1.Cmd != "SET_ACTIVITY" {
		t.Errorf("first frame cmd = %q, want SET_ACTIVITY", payload1.Cmd)
	}
	if payload1.Args.Activity != nil {
		t.Errorf("first frame activity should be nil (clear), got %+v", payload1.Args.Activity)
	}

	// Second frame: opClose.
	remaining := data[8+len1:]
	if len(remaining) < 8 {
		t.Fatalf("expected second frame, only %d bytes remaining", len(remaining))
	}
	op2 := binary.LittleEndian.Uint32(remaining[0:4])
	if op2 != opClose {
		t.Errorf("second frame opcode = %d, want %d (opClose)", op2, opClose)
	}

	// Verify Presence state is cleaned up.
	if p.connected {
		t.Error("expected connected=false after Close()")
	}
	if p.conn != nil {
		t.Error("expected conn=nil after Close()")
	}
}

func TestPresenceCloseWhenNotConnected(t *testing.T) {
	p := &Presence{}
	if err := p.Close(); err != nil {
		t.Errorf("Close on unconnected Presence: %v", err)
	}
}

func TestPresenceUpdateWhenNotConnected(t *testing.T) {
	p := &Presence{}
	if err := p.Update(PresenceState{TaskID: "TASK-001"}); err != nil {
		t.Errorf("Update on unconnected Presence: %v", err)
	}
}

func TestFormatState(t *testing.T) {
	tests := []struct {
		name  string
		state PresenceState
		want  string
	}{
		{
			name:  "empty verb falls back to Running Maggus",
			state: PresenceState{},
			want:  "Running Maggus",
		},
		{
			name:  "verb only no progress",
			state: PresenceState{Verb: "Consulting"},
			want:  "Consulting",
		},
		{
			name:  "verb with progress",
			state: PresenceState{Verb: "Working", ProgressCurrent: 3, ProgressTotal: 7},
			want:  "Working — 3/7 tasks (42%)",
		},
		{
			name:  "verb with full progress",
			state: PresenceState{Verb: "Working", ProgressCurrent: 1, ProgressTotal: 1},
			want:  "Working — 1/1 tasks (100%)",
		},
		{
			name:  "verb with zero current",
			state: PresenceState{Verb: "Fixing", ProgressCurrent: 0, ProgressTotal: 5},
			want:  "Fixing — 0/5 tasks (0%)",
		},
		{
			name:  "zero total means no progress",
			state: PresenceState{Verb: "Planning", ProgressCurrent: 0, ProgressTotal: 0},
			want:  "Planning",
		},
		{
			name:  "empty verb with progress falls back",
			state: PresenceState{ProgressCurrent: 2, ProgressTotal: 4},
			want:  "Running Maggus — 2/4 tasks (50%)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatState(tt.state)
			if got != tt.want {
				t.Errorf("formatState() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildActivityWithVerb(t *testing.T) {
	state := PresenceState{
		TaskID:    "TASK-001",
		TaskTitle: "Test",
		Verb:      "Fixing",
	}
	a := buildActivity(state)
	if a.State != "Fixing" {
		t.Errorf("State = %q, want %q", a.State, "Fixing")
	}
}

func TestBuildActivityWithVerbAndProgress(t *testing.T) {
	state := PresenceState{
		TaskID:          "TASK-001",
		TaskTitle:       "Test",
		Verb:            "Working",
		ProgressCurrent: 3,
		ProgressTotal:   7,
	}
	a := buildActivity(state)
	want := "Working — 3/7 tasks (42%)"
	if a.State != want {
		t.Errorf("State = %q, want %q", a.State, want)
	}
}
