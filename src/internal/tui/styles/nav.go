package styles

// CursorUp moves the cursor up by one with wraparound.
// When cursor is 0, it wraps to count-1.
// Returns 0 if count is 0.
func CursorUp(cursor, count int) int {
	if count == 0 {
		return 0
	}
	if cursor <= 0 {
		return count - 1
	}
	return cursor - 1
}

// CursorDown moves the cursor down by one with wraparound.
// When cursor is count-1, it wraps to 0.
// Returns 0 if count is 0.
func CursorDown(cursor, count int) int {
	if count == 0 {
		return 0
	}
	if cursor >= count-1 {
		return 0
	}
	return cursor + 1
}

// ClampCursor constrains cursor to [0, count-1].
// Returns 0 for empty lists (count == 0) or negative cursor.
func ClampCursor(cursor, count int) int {
	if count == 0 || cursor < 0 {
		return 0
	}
	if cursor >= count {
		return count - 1
	}
	return cursor
}
