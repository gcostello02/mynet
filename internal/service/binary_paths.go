package service

import "runtime"

// DarwinSystemBinaryDir is the root-owned directory for the supervised daemon binary.
const DarwinSystemBinaryDir = "/usr/local/libexec/mynet"

// DarwinSystemBinaryPath is the stable launchd ProgramArguments executable on macOS.
const DarwinSystemBinaryPath = DarwinSystemBinaryDir + "/mynet"

// DefaultSupervisedBinaryPath returns the recommended privileged install path for the OS.
func DefaultSupervisedBinaryPath() string {
	if runtime.GOOS == "darwin" {
		return DarwinSystemBinaryPath
	}
	return ""
}
