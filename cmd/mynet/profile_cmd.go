package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/grantcostello/mynet/internal/daemon"
)

func cmdProfile(args []string) {
	if len(args) < 1 {
		profileUsage()
		os.Exit(2)
	}
	dir := stateDir()
	switch args[0] {
	case "status":
		cmdProfileStatus(dir)
	case "cpu":
		fs := flag.NewFlagSet("profile cpu", flag.ExitOnError)
		secs := fs.Int("seconds", 30, "CPU profile duration")
		out := fs.String("o", "", "output file (default: stdout)")
		_ = fs.Parse(args[1:])
		cmdProfileCPU(dir, *secs, *out)
	case "snapshot":
		fs := flag.NewFlagSet("profile snapshot", flag.ExitOnError)
		kind := fs.String("kind", "heap", "heap|allocs|goroutine|mutex|block")
		out := fs.String("o", "", "output file (required)")
		_ = fs.Parse(args[1:])
		if *out == "" {
			fmt.Fprintln(os.Stderr, "profile snapshot requires -o")
			os.Exit(2)
		}
		cmdProfileSnapshot(dir, *kind, *out)
	case "fetch":
		fs := flag.NewFlagSet("profile fetch", flag.ExitOnError)
		path := fs.String("path", "/debug/pprof/profile", "pprof HTTP path (include ?seconds=30 for CPU)")
		out := fs.String("o", "", "output file (required)")
		_ = fs.Parse(args[1:])
		if *out == "" {
			fmt.Fprintln(os.Stderr, "profile fetch requires -o")
			os.Exit(2)
		}
		cmdProfileFetch(dir, *path, *out)
	default:
		profileUsage()
		os.Exit(2)
	}
}

func cmdProfileStatus(dir string) {
	if !daemon.DaemonReachable(dir) {
		fmt.Println("MyNet daemon not running.")
		return
	}
	resp, err := daemon.CallIPC(dir, "profile_status")
	if err != nil {
		fatal(err)
	}
	if !resp.OK || resp.Profile == nil {
		fatal(fmt.Errorf("%s", resp.Error))
	}
	p := resp.Profile
	fmt.Printf("Profiling enabled: %v\n", p.Enabled)
	if p.Enabled {
		fmt.Printf("Pprof socket:      %s\n", p.SocketPath)
		fmt.Printf("Profile directory: %s\n", filepath.Join(dir, "profiles"))
		fmt.Println()
		fmt.Println("Enable at daemon start: MYNET_ENABLE_PPROF=1")
		fmt.Println("Optional: MYNET_PPROF_MUTEX=1  MYNET_PPROF_BLOCK=1")
	} else {
		fmt.Println("Restart the daemon with MYNET_ENABLE_PPROF=1 to enable profiling.")
	}
}

func cmdProfileCPU(dir string, secs int, out string) {
	if !daemon.DaemonReachable(dir) {
		fatal(fmt.Errorf("daemon not running"))
	}
	resp, err := daemon.CallIPCRequest(dir, daemon.Request{Method: "profile_cpu", Seconds: secs})
	if err != nil {
		fatal(err)
	}
	if !resp.OK || resp.Profile == nil || resp.Profile.OutputPath == "" {
		fatal(fmt.Errorf("%s", resp.Error))
	}
	if err := copyProfileOut(resp.Profile.OutputPath, out); err != nil {
		fatal(err)
	}
	if out != "" {
		fmt.Printf("Wrote %s (from %s)\n", out, resp.Profile.OutputPath)
	}
}

func cmdProfileSnapshot(dir string, kind, out string) {
	resp, err := daemon.CallIPCRequest(dir, daemon.Request{Method: "profile_write", Profile: kind})
	if err != nil {
		fatal(err)
	}
	if !resp.OK || resp.Profile == nil || resp.Profile.OutputPath == "" {
		fatal(fmt.Errorf("%s", resp.Error))
	}
	if err := copyProfileOut(resp.Profile.OutputPath, out); err != nil {
		fatal(err)
	}
}

func cmdProfileFetch(dir string, pprofPath, out string) {
	resp, err := daemon.CallIPC(dir, "profile_status")
	if err != nil {
		fatal(err)
	}
	if !resp.OK || resp.Profile == nil || !resp.Profile.Enabled {
		fatal(fmt.Errorf("profiling not enabled"))
	}
	timeout := 35 * time.Second
	if containsSubstr(pprofPath, "seconds=") {
		timeout = 2 * time.Minute
	}
	data, err := daemon.FetchPprofOverUnix(resp.Profile.SocketPath, pprofPath, timeout)
	if err != nil {
		fatal(err)
	}
	if err := os.WriteFile(out, data, 0o600); err != nil {
		fatal(err)
	}
	fmt.Printf("Wrote %s (%d bytes)\n", out, len(data))
}

func copyProfileOut(src, out string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	if out == "" {
		_, err = io.Copy(os.Stdout, f)
		return err
	}
	w, err := os.Create(out)
	if err != nil {
		return err
	}
	if _, err = io.Copy(w, f); err != nil {
		_ = w.Close()
		return err
	}
	return w.Close()
}

func containsSubstr(s, sub string) bool {
	return len(sub) > 0 && strings.Index(s, sub) >= 0
}

func profileUsage() {
	fmt.Println(`Usage:
  mynet profile status
  mynet profile cpu [--seconds 30] [-o cpu.prof]
  mynet profile snapshot --kind heap|allocs|goroutine|mutex|block -o heap.prof
  mynet profile fetch --path '/debug/pprof/profile?seconds=30' -o cpu.prof`)
}
