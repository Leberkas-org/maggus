package logging

import (
	"strings"
	"time"
)

// FormatSlogLine parses a slog TextHandler line and returns a compact display string.
// Input:  time=2026-04-19T21:00:04.119+02:00 level=INFO msg="approve requested" component=tui pid=64504 item_id=fd700b64
// Output: 21:00:04.119 [INF] [tui:64504] approve requested item_id=fd700b64
func FormatSlogLine(line string) string {
	fields := parseSlogFields(line)
	if len(fields) == 0 {
		return line
	}

	var b strings.Builder

	// Time
	if t, ok := fields["time"]; ok {
		if parsed, err := time.Parse(time.RFC3339Nano, t); err == nil {
			b.WriteString(parsed.Format("15:04:05.000"))
		} else {
			b.WriteString(t)
		}
	}

	// Level
	b.WriteByte(' ')
	switch fields["level"] {
	case "DEBUG":
		b.WriteString("[DBG]")
	case "INFO":
		b.WriteString("[INF]")
	case "WARN":
		b.WriteString("[WRN]")
	case "ERROR":
		b.WriteString("[ERR]")
	default:
		b.WriteString("[???]")
	}

	// Component + PID
	comp := fields["component"]
	pid := fields["pid"]
	if comp != "" || pid != "" {
		b.WriteString(" [")
		if comp != "" {
			b.WriteString(comp)
		}
		if pid != "" {
			b.WriteByte(':')
			b.WriteString(pid)
		}
		b.WriteByte(']')
	}

	// Message
	b.WriteByte(' ')
	b.WriteString(fields["msg"])

	// Remaining attrs
	skip := map[string]bool{"time": true, "level": true, "msg": true, "component": true, "pid": true}
	for _, kv := range parseSlogFieldsOrdered(line) {
		if skip[kv[0]] {
			continue
		}
		b.WriteByte(' ')
		b.WriteString(kv[0])
		b.WriteByte('=')
		b.WriteString(kv[1])
	}

	return b.String()
}

func parseSlogFields(line string) map[string]string {
	fields := make(map[string]string)
	for _, kv := range parseSlogFieldsOrdered(line) {
		fields[kv[0]] = kv[1]
	}
	return fields
}

func parseSlogFieldsOrdered(line string) [][2]string {
	var result [][2]string
	remaining := line

	for remaining != "" {
		remaining = strings.TrimLeft(remaining, " ")
		if remaining == "" {
			break
		}

		eqIdx := strings.IndexByte(remaining, '=')
		if eqIdx < 0 {
			break
		}
		key := remaining[:eqIdx]
		remaining = remaining[eqIdx+1:]

		var value string
		if len(remaining) > 0 && remaining[0] == '"' {
			// Quoted value
			end := 1
			for end < len(remaining) {
				if remaining[end] == '\\' {
					end += 2
					continue
				}
				if remaining[end] == '"' {
					end++
					break
				}
				end++
			}
			value = remaining[1 : end-1]
			remaining = remaining[end:]
		} else {
			// Unquoted value
			spIdx := strings.IndexByte(remaining, ' ')
			if spIdx < 0 {
				value = remaining
				remaining = ""
			} else {
				value = remaining[:spIdx]
				remaining = remaining[spIdx:]
			}
		}

		result = append(result, [2]string{key, value})
	}
	return result
}
