package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/grantcostello/mynet/internal/config"
	"github.com/grantcostello/mynet/internal/service"
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
		bin := fs.String("binary", "", "mynet binary path")
		dry := fs.Bool("dry-run", false, "validate only; do not write system files")
		_ = fs.Parse(rest)
		spec, err := service.Install(service.InstallOptions{
			BinaryPath: *bin,
			StateDir:   *stateDir,
			OwnerUser:  *owner,
			DryRun:     *dry,
		})
		if err != nil {
			fatal(err)
		}
		fmt.Printf("Service install planned/completed (%s)\n", spec.Supervisor)
		fmt.Printf("  binary:    %s\n", spec.BinaryPath)
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
		live := fs.String("live", "bin/mynet", "installed binary path")
		candidate := fs.String("candidate", "", "new binary path (required)")
		backup := fs.String("backup", "", "rollback backup path")
		stageOnly := fs.Bool("stage-only", false, "only copy candidate to .next")
		_ = fs.Parse(rest)
		if *candidate == "" {
			fatal(fmt.Errorf("--candidate is required"))
		}
		plan, err := service.PlanUpgrade(*live, *candidate, *backup)
		if err != nil {
			fatal(err)
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
			fmt.Println("Stop daemon, then: mynet service upgrade --commit (or deploy script)")
			return
		}
		fmt.Println("Upgrade staged. Stop supervised daemon, run commit via install script or manual mv.")
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

  install   [--state-dir PATH] [--user USER] [--binary PATH] [--dry-run]
  uninstall [--dry-run]
  start | stop | restart   (requires sudo)
  status
  logs      [-f]
  upgrade   --candidate PATH [--live PATH] [--backup PATH] [--stage-only]`)
}

func serviceLocalSpecPath() string {
	sd, err := config.ResolveStateDir()
	if err != nil {
		return service.LocalSpecPath()
	}
	return filepath.Join(sd, "service.json")
}
