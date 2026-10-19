package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/grantcostello/mynet/internal/config"
	"github.com/grantcostello/mynet/internal/daemon"
	"github.com/grantcostello/mynet/internal/service"
	"github.com/grantcostello/mynet/internal/tundev"
)

func stateDir() string {
	d, err := config.ResolveStateDir()
	if err != nil {
		return config.DefaultStateDir()
	}
	return d
}

func ensureDaemon(log *slog.Logger) error {
	dir := stateDir()
	_ = daemon.RemoveStaleSocket(dir)
	sock := daemon.SocketPath(dir)
	if daemon.DaemonReachable(dir) {
		return nil
	}
	if ok, msg := service.ShouldSpawnDetachedDaemon(dir, serviceLocalSpecPath()); !ok {
		return fmt.Errorf("%s", msg)
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, "_daemon")
	configureDaemonSubprocess(cmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), config.EnvStateDir+"="+dir)
	if err := cmd.Start(); err != nil {
		return err
	}
	for i := 0; i < 50; i++ {
		if daemon.DaemonReachable(dir) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("daemon did not start (socket %s)", sock)
}

func runDaemon(log *slog.Logger) {
	dir := stateDir()
	node := &daemon.Node{StateDir: dir, Logger: log}
	if err := daemon.WritePIDFile(dir); err != nil {
		log.Error("pid file", "err", err)
		if config.Supervised() {
			os.Exit(1)
		}
	}

	if config.RunVPNAtStart() || config.Supervised() {
		go func() {
			log.Info("supervised VPN up starting", "state_dir", dir)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			if err := node.Up(ctx); err != nil {
				log.Error("supervised VPN up failed", "err", err)
				if config.Supervised() {
					os.Exit(1)
				}
				return
			}
			log.Info("supervised VPN up complete", "state_dir", dir)
		}()
	}

	go handleDaemonSignals(log)

	if err := daemon.ServeIPC(dir, node); err != nil {
		log.Error("daemon exit", "err", err)
		os.Exit(1)
	}
	os.Exit(0)
}

func handleDaemonSignals(log *slog.Logger) {
	ch := make(chan os.Signal, 2)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT)
	sig := <-ch
	log.Info("signal received", "signal", sig.String())
	if daemon.GracefulShutdownFromSignal(30 * time.Second) {
		os.Exit(0)
	}
	os.Exit(1)
}

func cmdUp(log *slog.Logger) {
	if err := ensureDaemon(log); err != nil {
		fatal(err)
	}
	dir := stateDir()
	resp, err := daemon.CallIPC(dir, "up")
	if err != nil {
		_ = daemon.RequestQuit(dir, daemon.DefaultQuitWait)
		fatal(err)
	}
	if !resp.OK {
		_ = daemon.RequestQuit(dir, daemon.DefaultQuitWait)
		fatal(fmt.Errorf("%s", resp.Error))
	}
	if resp.Status != nil {
		printVPNStatus(*resp.Status)
	}
}

func cmdDown(log *slog.Logger) {
	dir := stateDir()
	if !daemon.DaemonReachable(dir) {
		fmt.Println("MyNet daemon not running.")
		return
	}
	resp, err := daemon.CallIPC(dir, "down")
	if err != nil {
		fatal(err)
	}
	if !resp.OK && resp.Error != "" {
		fatal(fmt.Errorf("%s", resp.Error))
	}
	if resp.Status != nil && resp.Status.LastLifecycleError != "" {
		fmt.Fprintf(os.Stderr, "Note: %s\n", resp.Status.LastLifecycleError)
	}
	if daemon.DaemonReachable(dir) {
		fmt.Println("MyNet VPN stopped. (Signaling daemon still running; use: mynet daemon stop)")
	} else {
		fmt.Println("MyNet VPN stopped.")
	}
}

func cmdDaemonStop(log *slog.Logger) {
	dir := stateDir()
	if !daemon.DaemonReachable(dir) {
		_ = daemon.RemoveStaleSocket(dir)
		fmt.Println("MyNet daemon not running.")
		return
	}
	if service.SystemUnitInstalled(service.SystemLayoutForOS()) {
		fmt.Fprintln(os.Stderr, "Note: a system service is installed; prefer: sudo mynet service stop")
	}
	if err := daemon.RequestQuit(dir, daemon.DefaultQuitWait); err != nil {
		fatal(err)
	}
	fmt.Println("MyNet daemon stopped.")
}

func cmdVPNStatus(log *slog.Logger) {
	dir := stateDir()
	if !daemon.DaemonReachable(dir) {
		fmt.Println("MyNet daemon not running (or stale socket removed).")
		printPathDiagnostics()
		return
	}
	resp, err := daemon.CallIPC(dir, "status")
	if err != nil {
		fatal(err)
	}
	if resp.Status != nil {
		printVPNStatus(*resp.Status)
		return
	}
	fmt.Println("MyNet daemon not running or VPN down.")
}

func cmdPreflight(log *slog.Logger) {
	printPathDiagnostics()
	dir := stateDir()
	if err := config.IdentityMustExist(dir); err != nil {
		fatal(err)
	}
	// Preflight can run without daemon if we use local node
	node := &daemon.Node{StateDir: dir, Logger: log}
	if daemon.DaemonReachable(dir) {
		resp, err := daemon.CallIPC(dir, "preflight")
		if err != nil {
			fatal(err)
		}
		if !resp.OK {
			fatal(fmt.Errorf("%s", resp.Error))
		}
		if resp.Preflight != nil {
			if plan, ok := resp.Preflight.Plan.(tundev.Plan); ok {
				printVPNPlan(plan)
			} else {
				fmt.Println("VPN plan (from daemon):")
				fmt.Printf("  %v\n", resp.Preflight.Plan)
			}
		}
		return
	}
	plan, paths, err := node.Preflight()
	if err != nil {
		fatal(err)
	}
	printVPNPlan(plan)
	_ = paths
}

func printVPNPlan(plan tundev.Plan) {
	fmt.Println()
	fmt.Println("VPN plan (dry-run, not applied):")
	fmt.Printf("  TUN: %s\n", plan.TUNName)
	fmt.Printf("  Local: %s\n", plan.LocalPrefix)
	for _, r := range plan.PeerRoutes {
		fmt.Printf("  Route: %s\n", r)
	}
	for _, w := range plan.ConflictWarn {
		fmt.Printf("  Warning: %s\n", w)
	}
	if len(plan.PeerRoutes) == 0 {
		fmt.Println("  Peers: (none registered for VPN yet)")
	}
}

func printPathDiagnostics() {
	paths, err := config.CollectPreflightPaths()
	if err != nil {
		fatal(err)
	}
	fmt.Println("MyNet preflight — paths")
	fmt.Printf("  State dir:     %s\n", paths.StateDir)
	fmt.Printf("  Identity:      %s\n", paths.IdentityPath)
	fmt.Printf("  WireGuard key: %s\n", paths.WGKeyPath)
	fmt.Printf("  Client config: %s\n", paths.ClientConfig)
	fmt.Printf("  Session:       %s\n", paths.SessionPath)
	fmt.Printf("  Daemon socket: %s\n", paths.DaemonSocket)
	fmt.Printf("  EUID:          %d  USER=%s  SUDO_USER=%s\n", paths.EffectiveEUID, paths.EffectiveUser, paths.SudoInvoker)
	if paths.HomeFromGo != "" && paths.SudoInvoker != "" {
		fmt.Printf("  Note: under sudo, Go UserHomeDir() is %q; state uses invoker home via SUDO_USER.\n", paths.HomeFromGo)
	}
}

func printVPNStatus(st daemon.Status) {
	fmt.Println("MyNet")
	fmt.Println()
	fmt.Printf("Device: %s\n", st.DeviceName)
	fmt.Printf("Virtual IP: %s\n", st.VPNIP)
	fmt.Printf("Interface: %s\n", st.Interface)
	fmt.Println()
	fmt.Printf("Coordination: %s\n", st.Signaling)
	if st.SignalingLastError != "" {
		fmt.Printf("  Last error: %s\n", st.SignalingLastError)
	}
	if st.SignalingReconnect > 0 {
		fmt.Printf("  Reconnects: %d\n", st.SignalingReconnect)
	}
	if st.SignalingLastMsg != "" {
		fmt.Printf("  Last WS msg: %s\n", st.SignalingLastMsg)
	}
	fmt.Printf("WireGuard: %s\n", st.WireGuard)
	if st.DaemonPID > 0 {
		fmt.Printf("Daemon PID: %d", st.DaemonPID)
		if st.DaemonUptime != "" {
			fmt.Printf("  (uptime %s)", st.DaemonUptime)
		}
		fmt.Println()
	}
	if st.LastLifecycleError != "" {
		fmt.Printf("Last lifecycle note: %s\n", st.LastLifecycleError)
	}
	if len(st.Peers) > 0 {
		fmt.Println()
		fmt.Println("PEER     MYNET IP    TRANSPORT    ICE          STATE")
		for _, p := range st.Peers {
			ice := p.ICEConnState
			if ice == "" {
				ice = "—"
			}
			fmt.Printf("%-8s %-11s %-12s %-12s %s\n", p.Name, p.VPNIP, p.Transport, ice, p.State)
			if p.RecoveryState != "" && p.RecoveryState != "stable" {
				fmt.Printf("         recovery: %s (attempts %d)\n", p.RecoveryState, p.RecoveryAttempts)
			}
		}
	}
}
