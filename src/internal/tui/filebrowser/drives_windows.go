package filebrowser

import "os"

// availableDrives returns all drive letters that exist on the system (e.g., "C:\", "D:\").
func availableDrives() []string {
	var drives []string
	for c := 'A'; c <= 'Z'; c++ {
		drive := string(c) + `:\`
		if _, err := os.Stat(drive); err == nil {
			drives = append(drives, drive)
		}
	}
	return drives
}

// isDriveRoot returns true if the path is a Windows drive root (e.g., "C:\").
func isDriveRoot(dir string) bool {
	return len(dir) == 3 && dir[1] == ':' && (dir[2] == '\\' || dir[2] == '/')
}
