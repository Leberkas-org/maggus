package discord

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// IPC opcodes used by Discord's Rich Presence protocol.
const (
	opHandshake = 0
	opFrame     = 1
	opClose     = 2
)

// ipcHeader is the 8-byte header preceding every IPC message.
type ipcHeader struct {
	Opcode uint32
	Length uint32
}

// writeMessage sends a framed IPC message: 8-byte header (opcode + length) followed by the JSON payload.
func writeMessage(w io.Writer, opcode uint32, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	hdr := ipcHeader{Opcode: opcode, Length: uint32(len(data))}
	if err := binary.Write(w, binary.LittleEndian, hdr); err != nil {
		return fmt.Errorf("write header: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("write payload: %w", err)
	}
	return nil
}

// readMessage reads a framed IPC message and returns the opcode and raw JSON payload.
func readMessage(r io.Reader) (uint32, json.RawMessage, error) {
	var hdr ipcHeader
	if err := binary.Read(r, binary.LittleEndian, &hdr); err != nil {
		return 0, nil, fmt.Errorf("read header: %w", err)
	}

	data := make([]byte, hdr.Length)
	if _, err := io.ReadFull(r, data); err != nil {
		return 0, nil, fmt.Errorf("read payload: %w", err)
	}
	return hdr.Opcode, data, nil
}

// handshakePayload is the initial handshake sent to Discord.
type handshakePayload struct {
	V        int    `json:"v"`
	ClientID string `json:"client_id"`
}

// activityPayload is the SET_ACTIVITY command sent to Discord.
type activityPayload struct {
	Cmd   string       `json:"cmd"`
	Args  activityArgs `json:"args"`
	Nonce string       `json:"nonce"`
}

type activityArgs struct {
	PID      int       `json:"pid"`
	Activity *activity `json:"activity,omitempty"`
}

type activity struct {
	Details    string      `json:"details,omitempty"`
	State      string      `json:"state,omitempty"`
	Assets     *assets     `json:"assets,omitempty"`
	Timestamps *timestamps `json:"timestamps,omitempty"`
}

type assets struct {
	LargeImage string `json:"large_image,omitempty"`
	LargeText  string `json:"large_text,omitempty"`
}

type timestamps struct {
	Start *int64 `json:"start,omitempty"`
}

// formatState builds the state string from verb and progress fields.
func formatState(state PresenceState) string {
	verb := state.Verb
	if verb == "" {
		verb = "Running Maggus"
	}
	if state.ProgressTotal > 0 {
		pct := state.ProgressCurrent * 100 / state.ProgressTotal
		return fmt.Sprintf("%s \u2014 %d/%d tasks (%d%%)", verb, state.ProgressCurrent, state.ProgressTotal, pct)
	}
	return verb
}

// buildActivity constructs the activity payload from a PresenceState.
func buildActivity(state PresenceState) *activity {
	a := &activity{
		Details: FormatDetails(state),
		State:   formatState(state),
		Assets: &assets{
			LargeImage: AssetKeyLargeImage,
			LargeText:  "Maggus",
		},
	}

	if !state.StartTime.IsZero() {
		ms := state.StartTime.UnixMilli()
		a.Timestamps = &timestamps{Start: &ms}
	}

	return a
}

// FormatDetails formats the presence details line from a PresenceState.
// Output format: "FeatureTitle — TaskID: TaskTitle"
func FormatDetails(state PresenceState) string {
	if state.FeatureTitle == "" && state.TaskID == "" {
		return ""
	}
	if state.FeatureTitle == "" {
		return fmt.Sprintf("%s: %s", state.TaskID, state.TaskTitle)
	}
	if state.TaskID == "" {
		return state.FeatureTitle
	}
	return fmt.Sprintf("%s \u2014 %s: %s", state.FeatureTitle, state.TaskID, state.TaskTitle)
}

// buildSetActivityPayload creates the full SET_ACTIVITY command payload.
func buildSetActivityPayload(pid int, state PresenceState) activityPayload {
	return activityPayload{
		Cmd: "SET_ACTIVITY",
		Args: activityArgs{
			PID:      pid,
			Activity: buildActivity(state),
		},
		Nonce: fmt.Sprintf("%d", time.Now().UnixNano()),
	}
}

// buildClearActivityPayload creates a SET_ACTIVITY command with nil activity to clear presence.
func buildClearActivityPayload(pid int) activityPayload {
	return activityPayload{
		Cmd: "SET_ACTIVITY",
		Args: activityArgs{
			PID:      pid,
			Activity: nil,
		},
		Nonce: fmt.Sprintf("%d", time.Now().UnixNano()),
	}
}
