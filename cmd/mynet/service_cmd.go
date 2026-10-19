package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gcostello02/mynet/internal/config"
	"github.com/gcostello02/mynet/internal/service"
)

func cmdService(args []string) {
	if len(args) < 1 {
		serviceUsage()
		os.Exit(2)
	}
	sub := args[0]
	rest := args[1:]
	switch sub {
	case "install":
		fs := flag.NewFlagSet("service install", flag.ExitOnError)
		stateDir := fs.String("state-dir", "", "MyNet state directory (default: resolved ~/.mynet)")
		owner := fs.String("user", "", "identity owner username")
		bin := fs.String("binary", "", "mynet binary path (candidate to copy on macOS)")
		systemBin := fs.Bool("system-binary", runtime.GOOS == "darwin", "macOS: install copy into /usr/local/libexec/mynet/mynet")
		dry := fs.Bool("dry-run", false, "validate only; do not write system files")
		_ = fs.Parse(rest)
		spec, err := service.Install(service.InstallOptions{
			BinaryPath:   *bin,
			StateDir:     *stateDir,
			OwnerUser:    *owner,
			DryRun:       *dry,
			SystemBinary: *systemBin,
		})
		if err != nil {
			fatal(err)
		}
		fmt.Printf("Service install planned/completed (%s)\n", spec.Supervisor)
		fmt.Printf("  binary:    %s\n", spec.BinaryPath)
		if spec.SourceBinary != "" && spec.SourceBinary != spec.BinaryPath {
			fmt.Printf("  candidate: %s\n", spec.SourceBinary)
		}
		fmt.Printf("  state dir: %s\n", spec.StateDir)
		fmt.Printf("  owner:     %s\n", spec.OwnerUser)
		if *dry {
			fmt.Println("(dry-run: no system files written)")
		} else if runtime.GOOS == "darwin" {
			fmt.Println("Plist written; launchd is NOT loaded yet (daemon will not run until bootstrap).")
			fmt.Println("  verify: plutil -lint /Library/LaunchDaemons/com.mynet.daemon.plist")
			fmt.Println("  start:  sudo mynet service start   # launchctl bootstrap system …")
			fmt.Println("  note:   reboot loads plists under /Library/LaunchDaemons automatically")
		} else {
			fmt.Println("Next: sudo systemctl enable --now mynet-daemon  (or: mynet service start)")
		}
	case "uninstall":
		fs := flag.NewFlagSet("service uninstall", flag.ExitOnError)
		dry := fs.Bool("dry-run", false, "")
		_ = fs.Parse(rest)
		if err := service.Uninstall(*dry); err != nil {
			fatal(err)
		}
		fmt.Println("Service uninstalled (identity and state preserved).")
	case "start":
		if err := service.Start(); err != nil {
			fatal(err)
		}
		fmt.Println("Service start requested.")
	case "stop":
		if err := service.Stop(); err != nil {
			fatal(err)
		}
		fmt.Println("Service stop requested.")
	case "restart":
		if err := service.Restart(); err != nil {
			fatal(err)
		}
		fmt.Println("Service restart requested.")
	case "audit":
		sd, err := service.ResolveCLIStateDir()
		if err != nil {
			fatal(err)
		}
		audit := service.CollectInstallAudit(service.LocalSpecPath())
		fmt.Printf("Supervised binary: %s\n", audit.SupervisedBinary)
		fmt.Printf("State directory:   %s\n", sd)
		fmt.Println()
		fmt.Print(service.FormatSecurityFindings(audit.Findings))
	case "status":
		sd, err := service.ResolveCLIStateDir()
		if err != nil {
			fatal(err)
		}
		hr := service.CollectHealth(sd, service.LocalSpecPath())
		fmt.Println(service.FormatHealth(hr))
		if hr.DaemonReachable && hr.DaemonStatus != nil {
			printVPNStatus(*hr.DaemonStatus)
		}
	case "logs":
		fs := flag.NewFlagSet("service logs", flag.ExitOnError)
		follow := fs.Bool("f", false, "follow log output")
		_ = fs.Parse(rest)
		if err := service.Logs(*follow); err != nil {
			fatal(err)
		}
	case "upgrade":
		fs := flag.NewFlagSet("service upgrade", flag.ExitOnError)
		defaultLive := "bin/mynet"
		if p := service.ResolveUpgradeLivePath(service.LocalSpecPath()); p != "" {
			defaultLive = p
		}
		live := fs.String("live", defaultLive, "supervised binary path (launchd/systemd)")
		candidate := fs.String("candidate", "", "new binary path (required)")
		backup := fs.String("backup", "", "rollback backup path")
		stageOnly := fs.Bool("stage-only", false, "only copy candidate to .next")
		commit := fs.Bool("commit", false, "swap staged binary into live (requires sudo; daemon stopped)")
		_ = fs.Parse(rest)
		candPath := *candidate
		if candPath == "" {
			if *commit {
				candPath = *live
			} else {
				fatal(fmt.Errorf("--candidate is required"))
			}
		}
		plan, err := service.PlanUpgrade(*live, candPath, *backup)
		if err != nil {
			fatal(err)
		}
		if *commit {
			sd, err := service.ResolveCLIStateDir()
			if err != nil {
				fatal(err)
			}
			if err := service.CommitSwap(plan, sd); err != nil {
				fatal(err)
			}
			fmt.Printf("Upgrade committed: %s\n", plan.LivePath)
			fmt.Printf("Rollback copy:     %s\n", plan.BackupPath)
			return
		}
		if *candidate == "" {
			fatal(fmt.Errorf("--candidate is required"))
		}
		rev, err := binaryVCSRevision(*candidate)
		if err != nil {
			fatal(err)
		}
		fmt.Printf("Candidate revision: %s\n", rev)
		if err := service.StageCandidate(plan); err != nil {
			fatal(err)
		}
		fmt.Printf("Staged: %s\n", plan.StagedPath)
		if *stageOnly {
			fmt.Println("Next: sudo mynet service stop")
			fmt.Println("      sudo mynet service upgrade --commit --live", *live)
			return
		}
		fmt.Println("Upgrade staged. Stop supervised daemon, then: sudo mynet service upgrade --commit")
	default:
		serviceUsage()
		os.Exit(2)
	}
}

func binaryVCSRevision(path string) (string, error) {
	out, err := exec.Command("go", "version", "-m", path).Output()
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "vcs.revision=") {
			parts := strings.SplitN(line, "vcs.revision=", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1]), nil
			}
		}
	}
	return "", fmt.Errorf("no vcs.revision in %s", path)
}

func serviceUsage() {
	fmt.Println(`usage: mynet service <subcommand>

  install   [--state-dir PATH] [--user USER] [--binary PATH] [--system-binary] [--dry-run]
  audit
  uninstall [--dry-run]
  start | stop | restart   (requires sudo)
  status
  logs      [-f]
  upgrade   --candidate PATH [--live PATH] [--backup PATH] [--stage-only]
            upgrade --commit [--live PATH] [--backup PATH]  (after stop)`)
}

func serviceLocalSpecPath() string {
	sd, err := config.ResolveStateDir()
	if err != nil {
		return service.LocalSpecPath()
	}
	return filepath.Join(sd, "service.json")
}
