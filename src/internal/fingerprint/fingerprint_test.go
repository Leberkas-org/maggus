package fingerprint

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateUUID(t *testing.T) {
	id, err := generateUUID()
	if err != nil {
		t.Fatalf("generateUUID() error: %v", err)
	}
	if !isValidUUID(id) {
		t.Errorf("generated UUID %q is not valid", id)
	}
	// Check version nibble is 4.
	if id[14] != '4' {
		t.Errorf("UUID version nibble = %c, want '4'", id[14])
	}
	// Check variant nibble is 8, 9, a, or b.
	v := id[19]
	if v != '8' && v != '9' && v != 'a' && v != 'b' {
		t.Errorf("UUID variant nibble = %c, want 8/9/a/b", v)
	}
}

func TestGenerateUUID_Unique(t *testing.T) {
	a, _ := generateUUID()
	b, _ := generateUUID()
	if a == b {
		t.Error("two generated UUIDs should not be equal")
	}
}

func TestIsValidUUID(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"a1b2c3d4-e5f6-7890-abcd-ef1234567890", true},
		{"A1B2C3D4-E5F6-7890-ABCD-EF1234567890", true},
		{"00000000-0000-0000-0000-000000000000", true},
		{"", false},
		{"not-a-uuid", false},
		{"a1b2c3d4e5f6-7890-abcd-ef1234567890", false},  // missing first dash
		{"a1b2c3d4-e5f6-7890-abcd-ef123456789", false},   // too short
		{"a1b2c3d4-e5f6-7890-abcd-ef12345678901", false}, // too long
		{"g1b2c3d4-e5f6-7890-abcd-ef1234567890", false},  // invalid hex char
	}
	for _, tt := range tests {
		if got := isValidUUID(tt.input); got != tt.want {
			t.Errorf("isValidUUID(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestGet_CreatesAndReads(t *testing.T) {
	dir := t.TempDir()
	sysDir := filepath.Join(dir, "system")
	userDir := filepath.Join(dir, "user")

	// First call: should create fingerprint.
	id, err := get(sysDir, userDir)
	if err != nil {
		t.Fatalf("get() error: %v", err)
	}
	if !isValidUUID(id) {
		t.Errorf("returned ID %q is not a valid UUID", id)
	}

	// File should exist in system dir.
	data, err := os.ReadFile(filepath.Join(sysDir, "fingerprint"))
	if err != nil {
		t.Fatalf("reading system fingerprint: %v", err)
	}
	if got := string(data); got != id+"\n" {
		t.Errorf("file content = %q, want %q", got, id+"\n")
	}

	// Second call: should return the same ID.
	id2, err := get(sysDir, userDir)
	if err != nil {
		t.Fatalf("get() second call error: %v", err)
	}
	if id2 != id {
		t.Errorf("second call returned %q, want %q", id2, id)
	}
}

func TestGet_FallbackToUserDir(t *testing.T) {
	dir := t.TempDir()
	// Use a path that cannot be created (file where directory expected)
	// to simulate system path write failure.
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	sysDir := filepath.Join(blocker, "system") // can't mkdir under a file
	userDir := filepath.Join(dir, "user")

	id, err := get(sysDir, userDir)
	if err != nil {
		t.Fatalf("get() error: %v", err)
	}
	if !isValidUUID(id) {
		t.Errorf("returned ID %q is not a valid UUID", id)
	}

	// Should be in user dir.
	if _, err := os.Stat(filepath.Join(userDir, "fingerprint")); err != nil {
		t.Errorf("expected fingerprint in user dir: %v", err)
	}
}

func TestGet_ReadsFromUserDir(t *testing.T) {
	dir := t.TempDir()
	sysDir := filepath.Join(dir, "system") // does not exist
	userDir := filepath.Join(dir, "user")

	// Pre-write a fingerprint in user dir.
	expected := "12345678-1234-4234-8234-123456789abc"
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userDir, "fingerprint"), []byte(expected+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	id, err := get(sysDir, userDir)
	if err != nil {
		t.Fatalf("get() error: %v", err)
	}
	if id != expected {
		t.Errorf("got %q, want %q", id, expected)
	}
}

func TestGet_PrefersSystemOverUser(t *testing.T) {
	dir := t.TempDir()
	sysDir := filepath.Join(dir, "system")
	userDir := filepath.Join(dir, "user")

	sysID := "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	userID := "11111111-2222-4333-8444-555555555555"

	for _, d := range []struct {
		dir string
		id  string
	}{
		{sysDir, sysID},
		{userDir, userID},
	} {
		if err := os.MkdirAll(d.dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d.dir, "fingerprint"), []byte(d.id+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	id, err := get(sysDir, userDir)
	if err != nil {
		t.Fatalf("get() error: %v", err)
	}
	if id != sysID {
		t.Errorf("got %q, want system ID %q", id, sysID)
	}
}
