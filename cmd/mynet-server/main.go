package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gcostello02/mynet/internal/control/server"
)

func main() {
	fs := flag.NewFlagSet("mynet-server", flag.ExitOnError)
	listen := fs.String("listen", ":8080", "HTTP listen address")
	network := fs.String("network", "default", "logical network id")
	dataPath := fs.String("data", "", "SQLite path for enrolled devices (or MYNET_DATA_PATH)")
	_ = fs.Parse(os.Args[1:])
	if *dataPath == "" {
		*dataPath = os.Getenv("MYNET_DATA_PATH")
	}

	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	serverSecret := os.Getenv("MYNET_SERVER_SECRET")
	if serverSecret == "" {
		log.Error("MYNET_SERVER_SECRET is required (>= 32 bytes)")
		os.Exit(1)
	}
	enrollSecret := os.Getenv("MYNET_ENROLLMENT_SECRET")
	if enrollSecret == "" {
		// Development convenience: generate ephemeral secret and print once.
		b := make([]byte, 16)
		_, _ = rand.Read(b)
		enrollSecret = hex.EncodeToString(b)
		log.Warn("MYNET_ENROLLMENT_SECRET not set; using ephemeral secret (set env for stable enrollments)", "secret", enrollSecret)
	}

	turnSecret := os.Getenv("MYNET_TURN_SECRET")
	var iceCfg server.ICEConfig
	iceCfg.STUNURLs = server.ParseCSVEnv(os.Getenv("MYNET_STUN_URLS"))
	iceCfg.TURNURLs = server.ParseCSVEnv(os.Getenv("MYNET_TURN_URLS"))
	if turnSecret != "" {
		iceCfg.TURNSharedSecret = []byte(turnSecret)
		iceCfg.TURNCredentialTTL = 24 * time.Hour
		log.Info("TURN credentials enabled for clients", "turn_urls", iceCfg.TURNURLs)
	}

	vpnCIDR := os.Getenv("MYNET_VPN_CIDR")
	if vpnCIDR == "" {
		vpnCIDR = "172.31.240.0/20"
	}

	secureCookies := os.Getenv("MYNET_SECURE_COOKIES") == "1" || os.Getenv("MYNET_SECURE_COOKIES") == "true"
	srv, err := server.New(server.Config{
		ListenAddr:       *listen,
		ServerSecret:     []byte(serverSecret),
		EnrollmentSecret: enrollSecret,
		NetworkID:        *network,
		Logger:           log,
		ICE:              iceCfg,
		VPN:              server.VPNConfig{IPv4CIDR: vpnCIDR},
		DataPath:         *dataPath,
		SetupToken:       os.Getenv("MYNET_SETUP_TOKEN"),
		SecureCookies:    secureCookies,
	})
	if err != nil {
		log.Error("server init failed", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := srv.ListenAndServe(); err != nil && ctx.Err() == nil {
			log.Error("listen failed", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	log.Info("shutdown complete")
}
