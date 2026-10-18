package service

import (
	"fmt"
	"strings"

	"github.com/grantcostello/mynet/internal/config"
)

// LaunchDaemonPlist renders a system LaunchDaemon plist (no shell wrapper).
func LaunchDaemonPlist(binaryPath, stateDir, ownerUser, logPath string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
		<string>_daemon</string>
	</array>
	<key>EnvironmentVariables</key>
	<dict>
		<key>%s</key>
		<string>%s</string>
		<key>%s</key>
		<string>1</string>
		<key>%s</key>
		<string>1</string>
		<key>%s</key>
		<string>%s</string>
	</dict>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<dict>
		<key>SuccessfulExit</key>
		<false/>
	</dict>
	<key>StandardOutPath</key>
	<string>%s</string>
	<key>StandardErrorPath</key>
	<string>%s</string>
</dict>
</plist>
`, Label, binaryPath,
		config.EnvStateDir, stateDir,
		config.EnvSupervised,
		config.EnvRunVPNAtStart,
		config.EnvOwnerUser, ownerUser,
		logPath, logPath)
}

// ParseLaunchDaemonPlistLabel extracts Label for template tests.
func ParseLaunchDaemonPlistLabel(plist string) (string, bool) {
	const key = "<key>Label</key>"
	i := strings.Index(plist, key)
	if i < 0 {
		return "", false
	}
	rest := plist[i+len(key):]
	s := strings.Index(rest, "<string>")
	e := strings.Index(rest, "</string>")
	if s < 0 || e < 0 || e <= s {
		return "", false
	}
	return rest[s+len("<string>") : e], true
}
