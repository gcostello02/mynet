package service

import (
	"os"
	"runtime"
	"strings"
)

// InstallAudit aggregates security checks for the supervised install.
type InstallAudit struct {
	SupervisedBinary string
	Findings         []SecurityFinding
}

// CollectInstallAudit inspects local metadata and on-disk supervisor files.
func CollectInstallAudit(localSpecPath string) InstallAudit {
	layout := SystemLayoutForOS()
	binary := ResolveUpgradeLivePath(localSpecPath)
	if binary == "" {
		if p, ok := PlistBinaryFromLocalSpec(localSpecPath); ok {
			binary = p
		}
	}
	if binary == "" && runtime.GOOS == "darwin" {
		binary = binaryFromLaunchdPlist(layout.LaunchDaemonPlist)
	}
	var findings []SecurityFinding
	if binary != "" {
		bf, err := AuditSupervisedBinaryPath(binary)
		if err != nil {
			findings = append(findings, SecurityFinding{Level: "warn", Topic: "binary audit", Detail: err.Error()})
		} else {
			findings = append(findings, bf...)
		}
	} else {
		findings = append(findings, SecurityFinding{
			Level:  "warn",
			Topic:  "binary",
			Detail: "no supervised binary path found (service not installed or metadata missing)",
		})
	}
	if runtime.GOOS == "darwin" {
		findings = append(findings, auditLaunchdPlistFile(layout.LaunchDaemonPlist))
	}
	return InstallAudit{SupervisedBinary: binary, Findings: findings}
}

func auditLaunchdPlistFile(path string) SecurityFinding {
	fi, err := os.Lstat(path)
	if err != nil {
		return SecurityFinding{Level: "warn", Topic: "plist", Detail: "LaunchDaemon plist not present"}
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return SecurityFinding{Level: "critical", Topic: "plist", Detail: "LaunchDaemon plist must not be a symlink"}
	}
	mode := fi.Mode().Perm()
	if mode&0o022 != 0 {
		return SecurityFinding{Level: "critical", Topic: "plist", Detail: "LaunchDaemon plist must not be group/world writable"}
	}
	return SecurityFinding{Level: "ok", Topic: "plist", Detail: "LaunchDaemon plist present with safe permissions"}
}

func binaryFromLaunchdPlist(plistPath string) string {
	b, err := os.ReadFile(plistPath)
	if err != nil {
		return ""
	}
	const marker = "<key>ProgramArguments</key>"
	s := string(b)
	i := strings.Index(s, marker)
	if i < 0 {
		return ""
	}
	rest := s[i:]
	a := strings.Index(rest, "<array>")
	beg := strings.Index(rest, "<string>")
	if a < 0 || beg < 0 {
		return ""
	}
	rest = rest[beg+len("<string>"):]
	end := strings.Index(rest, "</string>")
	if end < 0 {
		return ""
	}
	return rest[:end]
}
