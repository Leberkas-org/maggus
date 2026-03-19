//go:build !windows

package filebrowser

// availableDrives is a no-op on non-Windows platforms.
func availableDrives() []string {
	return nil
}

// isDriveRoot always returns false on non-Windows platforms.
func isDriveRoot(_ string) bool {
	return false
}
