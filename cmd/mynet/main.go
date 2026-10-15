package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gcostello02/mynet/internal/client"
	"github.com/gcostello02/mynet/internal/config"
	"github.com/gcostello02/mynet/internal/control"
	"github.com/gcostello02/mynet/internal/daemon"
)

func main() {
	lvl := slog.LevelInfo
	if os.Getenv("MYNET_DEBUG") != "" {
		lvl = slog.LevelDebug
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl}))
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	rt := client.NewRuntime(stateDir(), log)
	cmd := os.Args[1]
	switch cmd {
	case "init":
		fs := flag.NewFlagSet("init", flag.ExitOnError)
		name := fs.String("name", "", "device name (required)")
		server := fs.String("server", "", "coordination server URL")
		tailnet := fs.Bool("tailnet", false, "allow Tailscale 100.x ICE candidates (auto if server URL is 100.x)")
		_ = fs.Parse(os.Args[2:])
		if *name == "" {
			fmt.Fprintln(os.Stderr, "--name is required")
			os.Exit(2)
		}
		if err := rt.Init(*name); err != nil {
			fatal(err)
		}
		cfg, _ := config.LoadClientConfig(stateDir())
		if *server != "" {
			cfg.ServerURL = *server
		}
		if *tailnet {
			cfg.AllowTailscaleCandidates = true
		}
		if *server != "" || *tailnet {
			if err := config.SaveClientConfig(stateDir(), cfg); err != nil {
				fatal(err)
			}
		}
		fmt.Printf("Initialized device %q in %s\n", *name, stateDir())
	case "login":
		fs := flag.NewFlagSet("login", flag.ExitOnError)
		enroll := fs.String("enrollment-secret", "", "network enrollment secret (required on first enroll)")
		stay := fs.Bool("stay", false, "keep signaling WebSocket open until Ctrl+C (use on both devices while testing)")
		_ = fs.Parse(os.Args[2:])
		secret := *enroll
		if secret == "" {
			secret = os.Getenv("MYNET_ENROLLMENT_SECRET")
		}
		if err := rt.Login(secret); err != nil {
			fatal(err)
		}
		fmt.Println("Logged in (session saved).")
		if *stay {
			fmt.Println("Staying online for signaling (Ctrl+C to exit).")
			ctx, cancel := context.WithCancel(context.Background())
			rt.StartSignaling(ctx)
			ch := make(chan os.Signal, 1)
			signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
			<-ch
			cancel()
			_ = rt.Down()
		}
	case "peers":
		if daemon.DaemonReachable(stateDir()) {
			resp, err := daemon.CallIPC(stateDir(), "peers")
			if err == nil && resp.OK {
				printPeerList(resp.Peers)
				return
			}
		}
		peers, err := rt.Peers()
		if err != nil {
			fatal(err)
		}
		printPeerList(peers)
	case "connect":
		fs := flag.NewFlagSet("connect", flag.ExitOnError)
		peer := fs.String("peer", "", "peer device name")
		relayOnly := fs.Bool("relay-only", false, "restrict ICE to TURN relay candidates (test mode)")
		_ = fs.Parse(os.Args[2:])
		if *peer == "" {
			fmt.Fprintln(os.Stderr, "--peer is required")
			os.Exit(2)
		}
		if err := rt.ConnectWithOptions(*peer, client.ConnectOptions{RelayOnly: *relayOnly}); err != nil {
			fatal(err)
		}
		rtt, err := rt.Ping(*peer)
		if err != nil {
			fatal(err)
		}
		st, _ := rt.Status()
		printStatus(st)
		fmt.Printf("Ping RTT: %s\n", formatRTT(rtt))
	case "ping":
		fs := flag.NewFlagSet("ping", flag.ExitOnError)
		peer := fs.String("peer", "", "peer device name")
		_ = fs.Parse(os.Args[2:])
		rtt, err := rt.Ping(*peer)
		if err != nil {
			fatal(err)
		}
		fmt.Printf("RTT: %s\n", formatRTT(rtt))
	case "status":
		if daemon.DaemonReachable(stateDir()) {
			cmdVPNStatus(log)
		} else {
			st, err := rt.Status()
			if err != nil {
				fatal(err)
			}
			printStatus(st)
		}
	case "preflight":
		cmdPreflight(log)
	case "vpn":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: mynet vpn register")
			os.Exit(2)
		}
		switch os.Args[2] {
		case "register":
			ip, err := rt.RegisterVPN()
			if err != nil {
				fatal(err)
			}
			fmt.Printf("VPN registered. Assigned address: %s/32\n", ip)
			fmt.Println("Next: mynet preflight")
		default:
			fmt.Fprintf(os.Stderr, "unknown vpn subcommand %q (try: mynet vpn register)\n", os.Args[2])
			os.Exit(2)
		}
	case "diagnose":
		fs := flag.NewFlagSet("diagnose", flag.ExitOnError)
		peer := fs.String("peer", "", "peer device name")
		_ = fs.Parse(os.Args[2:])
		if daemon.DaemonReachable(stateDir()) {
			resp, err := daemon.CallIPCWithPeer(stateDir(), "diagnose", *peer)
			if err == nil && resp.OK && resp.Diagnostics != nil {
				printDiagnostics(*resp.Diagnostics)
				return
			}
		}
		d, err := rt.Diagnose(*peer)
		if err != nil {
			fatal(err)
		}
		printDiagnostics(d)
	case "config":
		if len(os.Args) < 3 {
			configUsage()
			os.Exit(2)
		}
		stateDir := stateDir()
		cfg, err := config.LoadClientConfig(stateDir)
		if err != nil {
			fatal(err)
		}
		sub := os.Args[2]
		switch sub {
		case "show":
			printClientConfig(cfg)
		case "tailnet":
			fs := flag.NewFlagSet("config tailnet", flag.ExitOnError)
			server := fs.String("server", "", "coordination server URL (Tailscale IP or hostname)")
			_ = fs.Parse(os.Args[3:])
			if *server != "" {
				cfg.ServerURL = *server
			}
			cfg.AllowTailscaleCandidates = true
			if err := config.SaveClientConfig(stateDir, cfg); err != nil {
				fatal(err)
			}
			fmt.Println("Tailnet ICE mode enabled (100.x host candidates allowed).")
			printClientConfig(cfg)
		case "set-server":
			if len(os.Args) < 4 {
				fmt.Fprintln(os.Stderr, "usage: mynet config set-server <url>")
				os.Exit(2)
			}
			cfg.ServerURL = os.Args[3]
			if err := config.SaveClientConfig(stateDir, cfg); err != nil {
				fatal(err)
			}
			fmt.Println("Updated server URL.")
			printClientConfig(cfg)
		case "independent":
			fs := flag.NewFlagSet("config independent", flag.ExitOnError)
			server := fs.String("server", "", "public coordination server URL (https recommended)")
			_ = fs.Parse(os.Args[3:])
			if *server != "" {
				cfg.ServerURL = *server
			}
			cfg.Network.IndependentMode = true
			cfg.AllowTailscaleCandidates = false
			if len(cfg.Network.ExcludeCandidateCIDRs) == 0 {
				cfg.Network.ExcludeCandidateCIDRs = []string{config.DefaultExcludeTailscaleCIDR}
			}
			if len(cfg.Network.ExcludeInterfaces) == 0 {
				cfg.Network.ExcludeInterfaces = []string{"tailscale0"}
			}
			// EffectiveExcludeInterfaces merges DefaultIndependentExcludeInterfaces at runtime.
			if err := config.SaveClientConfig(stateDir, cfg); err != nil {
				fatal(err)
			}
			fmt.Println("Independent connectivity mode enabled (Tailscale ICE/signaling paths excluded in config).")
			printClientConfig(cfg)
		default:
			configUsage()
			os.Exit(2)
		}
	case "daemon":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: mynet daemon stop")
			os.Exit(2)
		}
		switch os.Args[2] {
		case "stop":
			cmdDaemonStop(log)
		default:
			fmt.Fprintf(os.Stderr, "unknown daemon subcommand %q (try: mynet daemon stop)\n", os.Args[2])
			os.Exit(2)
		}
	case "up":
		cmdUp(log)
	case "down":
		if len(os.Args) >= 3 && os.Args[2] == "--vpn" {
			cmdDown(log)
		} else if daemon.DaemonReachable(stateDir()) {
			cmdDown(log)
		} else if err := rt.Down(); err != nil {
			fatal(err)
		} else {
			fmt.Println("Disconnected.")
		}
	case "_daemon":
		runDaemon(log)
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", cmd)
		usage()
		os.Exit(2)
	}
}

func printStatus(st client.Status) {
	fmt.Println("MYNET")
	fmt.Println()
	fmt.Printf("Device: %s\n", st.DeviceName)
	if st.PeerName != "" {
		fmt.Printf("Peer:   %s\n", st.PeerName)
	}
	fmt.Println()
	fmt.Printf("Signaling:  %s\n", st.Signaling)
	fmt.Printf("State:      %s\n", st.State)
	if st.ICEState != "" && st.State == client.StateConnected {
		fmt.Printf("ICE State:  %s\n", st.ICEState)
	}
	if st.State == client.StateConnected {
		if st.Connection != "" {
			fmt.Printf("Connection: %s\n", st.Connection)
		}
		fmt.Printf("Transport:  %s\n", st.Transport)
		fmt.Printf("Local:      %s (%s)\n", st.Local, st.LocalType)
		fmt.Printf("Remote:     %s (%s)\n", st.Remote, st.RemoteType)
		fmt.Printf("TURN Used:  %v\n", st.TURNUsed)
		if st.RTT > 0 {
			fmt.Printf("RTT:        %s\n", formatRTT(st.RTT))
		}
	}
	if st.LastError != "" {
		fmt.Printf("Error:      %s\n", st.LastError)
	}
	fmt.Printf("Server:     %s\n", st.ServerURL)
}

func printPeerList(peers []control.PeerInfo) {
	if len(peers) == 0 {
		fmt.Println("No peers in network.")
		return
	}
	fmt.Println("PEERS")
	for _, p := range peers {
		status := "offline"
		if p.Online {
			status = "online"
		}
		fmt.Printf("  %-20s %s  %s\n", p.Name, p.DeviceID[:8]+"…", status)
	}
}

func printDiagnostics(d client.Diagnostics) {
	fmt.Println("MyNet — Connection Diagnostics")
	fmt.Println()
	fmt.Printf("Device: %s\n", d.DeviceName)
	if d.PeerName != "" {
		fmt.Printf("Peer:   %s\n", d.PeerName)
	}
	fmt.Println()
	fmt.Println("Coordination:")
	fmt.Printf("  Server:         %s\n", d.ServerURL)
	if d.CoordinationOK {
		fmt.Printf("  Health:         OK\n")
	}
	fmt.Printf("  Authentication: %s\n", d.Authentication)
	fmt.Printf("  Signaling:      %s\n", d.Signaling)
	if d.SignalingReconnect > 0 {
		fmt.Printf("  Reconnects:     %d\n", d.SignalingReconnect)
	}
	if d.SignalingLastMsg != "" {
		fmt.Printf("  Last WS msg:    %s\n", d.SignalingLastMsg)
	}
	if d.WireGuardState != "" {
		fmt.Printf("  WireGuard:      %s\n", d.WireGuardState)
	}
	if d.PeerName != "" {
		fmt.Printf("  Peer online:    %v\n", d.PeerOnline)
	}
	if d.IndependentMode {
		fmt.Printf("  Mode:           independent\n")
	}
	fmt.Println()
	fmt.Println("ICE:")
	fmt.Printf("  State:            %s\n", d.ICEState)
	if d.ICEGatherDuration > 0 {
		fmt.Printf("  Gather duration:  %s\n", d.ICEGatherDuration.Round(time.Millisecond))
	}
	if d.LocalCandidateCounts.Total > 0 || d.RemoteCandidateCounts.Total > 0 {
		fmt.Printf("  Local candidates:  host=%d srflx=%d relay=%d (total %d)\n",
			d.LocalCandidateCounts.Host, d.LocalCandidateCounts.Srflx, d.LocalCandidateCounts.Relay, d.LocalCandidateCounts.Total)
		fmt.Printf("  Remote candidates: host=%d srflx=%d relay=%d (total %d)\n",
			d.RemoteCandidateCounts.Host, d.RemoteCandidateCounts.Srflx, d.RemoteCandidateCounts.Relay, d.RemoteCandidateCounts.Total)
	}
	if len(d.ICEExcludeInterfaces) > 0 {
		fmt.Printf("  Excluded ifaces:  %v\n", d.ICEExcludeInterfaces)
	}
	if len(d.ICEExcludeCIDRs) > 0 {
		fmt.Printf("  Excluded CIDRs:   %v\n", d.ICEExcludeCIDRs)
	}
	if len(d.ICEActiveServers) > 0 {
		fmt.Printf("  ICE servers:      %v\n", d.ICEActiveServers)
	}
	if d.Connection != "" {
		fmt.Printf("  Path:             %s\n", d.Connection)
		fmt.Printf("  Local Candidate:  %s\n", d.LocalType)
		fmt.Printf("  Remote Candidate: %s\n", d.RemoteType)
	}
	fmt.Println()
	fmt.Println("Transport:")
	if d.LocalAddr != "" {
		fmt.Printf("  Local:  %s\n", d.LocalAddr)
	}
	if d.RemoteAddr != "" {
		fmt.Printf("  Remote: %s\n", d.RemoteAddr)
	}
	if d.RTT > 0 {
		fmt.Printf("  RTT:    %s\n", formatRTT(d.RTT))
	}
	fmt.Printf("  TURN Used: %v\n", d.TURNUsed)
	if d.LastComm != "" {
		fmt.Printf("  Last ping: %s\n", d.LastComm)
	}
	fmt.Println()
	fmt.Println("Tailscale Dependency:")
	fmt.Printf("  Signaling: %s\n", d.TailscaleSignaling)
	fmt.Printf("  Data Path: %s\n", d.TailscaleDataPath)
	if d.LastError != "" {
		fmt.Println()
		fmt.Printf("Last error: %s\n", d.LastError)
	}
	if len(d.LocalCandidates) > 0 {
		fmt.Println()
		fmt.Println("Local ICE candidates:")
		for _, c := range d.LocalCandidates {
			fmt.Printf("  %-28s %s\n", c.Type, c.Address)
		}
	}
	if len(d.RemoteCandidates) > 0 {
		fmt.Println()
		fmt.Println("Remote ICE candidates:")
		for _, c := range d.RemoteCandidates {
			fmt.Printf("  %-28s %s\n", c.Type, c.Address)
		}
	}
	if len(d.Warnings) > 0 {
		fmt.Println()
		fmt.Println("Warnings:")
		for _, w := range d.Warnings {
			fmt.Printf("  - %s\n", w)
		}
	}
	if d.LastError != "" {
		fmt.Printf("\nLast error:      %s\n", d.LastError)
	}
}

func configUsage() {
	fmt.Println(`Usage:
  mynet config show
  mynet config tailnet [--server <url>]
  mynet config independent [--server <url>]
  mynet config set-server <url>`)
}

func printClientConfig(cfg config.ClientConfig) {
	fmt.Printf("server_url: %s\n", cfg.ServerURL)
	fmt.Printf("independent_mode: %v\n", cfg.Network.IndependentMode)
	fmt.Printf("allow_tailscale_candidates: %v (effective tailnet ICE: %v)\n", cfg.AllowTailscaleCandidates, cfg.ICESessionAllowTailscale())
	if len(cfg.Network.ExcludeCandidateCIDRs) > 0 {
		fmt.Printf("exclude_candidate_cidrs: %v\n", cfg.Network.ExcludeCandidateCIDRs)
	}
	if len(cfg.Network.ExcludeInterfaces) > 0 {
		fmt.Printf("exclude_interfaces: %v\n", cfg.Network.ExcludeInterfaces)
	}
	if len(cfg.STUNURLs) > 0 {
		fmt.Printf("stun_urls: %v\n", cfg.STUNURLs)
	}
	if len(cfg.TURNURLs) > 0 {
		fmt.Printf("turn_urls: %v\n", cfg.TURNURLs)
	}
	if len(cfg.ICEURLs) > 0 {
		fmt.Printf("ice_urls: %v\n", cfg.ICEURLs)
	}
	fmt.Printf("ice_urls_effective: %v\n", cfg.EffectiveICEURLs())
}

func formatRTT(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
	if d < 10*time.Millisecond {
		return fmt.Sprintf("%.2fms", float64(d)/float64(time.Millisecond))
	}
	return d.Round(time.Millisecond).String()
}

func usage() {
	fmt.Println(`MyNet V0.2.2 — WireGuard mesh VPN over ICE

Usage:
  mynet init --name <device> [--server <url>] [--tailnet]
  mynet login [--enrollment-secret <secret>] [--stay]
  mynet vpn register
  mynet peers
  mynet connect --peer <name> [--relay-only]
  mynet ping [--peer <name>]
  mynet status
  mynet diagnose [--peer <name>]
  mynet config show | tailnet | independent | set-server
  mynet preflight
  mynet up
  mynet down          (stops VPN; daemon may keep running)
  mynet daemon stop   (exit signaling daemon)

Environment:
  MYNET_ENROLLMENT_SECRET  enrollment secret for first login`)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
