package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/lumina-proxy/peer-agent/internal/agent"
	"github.com/lumina-proxy/peer-agent/internal/hubca"
)

const defaultHub = "peers.luminaproxy.com:443"

const maxLogBytes = 10 << 20

const consentText = `LuminaProxy Peer shares part of your internet connection with paying
customers of the LuminaProxy network. While it runs, web requests from those
customers are made from your device's connection. It never touches your files
or your own browsing, and it refuses connections to private or local addresses.
You can pause or uninstall it at any time.`

var version = "dev"

func main() {
	var (
		hubAddr = flag.String("hub", env("HUB_ADDR", defaultHub), "hub control address host:port")
		token   = flag.String("token", os.Getenv("DEVICE_TOKEN"), "device token")
		caFile  = flag.String("ca", os.Getenv("HUB_CA_FILE"), "PEM file with the hub's CA certificate (default: built in)")
		sni     = flag.String("sni", env("HUB_SNI", "peer-hub.luminaproxy"), "expected hub TLS server name")
		pauseF  = flag.String("pause-file", os.Getenv("PAUSE_FILE"), "agent pauses while this file exists")
		logFile = flag.String("log-file", os.Getenv("LOG_FILE"), "write logs to this file instead of stdout")
		consent = flag.Bool("i-consent", os.Getenv("LUMINA_PEER_CONSENT") == "yes", "confirm you consent to sharing bandwidth")
		showVer = flag.Bool("version", false, "print the version and exit")
	)
	flag.Parse()

	if *showVer {
		fmt.Println(version)
		return
	}

	out, err := logOutput(*logFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot open log file:", err)
		os.Exit(1)
	}
	log := slog.New(slog.NewTextHandler(out, &slog.HandlerOptions{Level: logLevel()}))

	if !*consent {
		fmt.Fprintln(os.Stderr, consentText)
		fmt.Fprintln(os.Stderr, "\nRe-run with --i-consent (or set LUMINA_PEER_CONSENT=yes) to start sharing.")
		os.Exit(2)
	}
	if *hubAddr == "" || *token == "" {
		log.Error("a device token is required (--token or DEVICE_TOKEN)")
		os.Exit(1)
	}

	caPEM := hubca.PEM
	if *caFile != "" {
		caPEM, err = os.ReadFile(*caFile)
		if err != nil {
			log.Error("cannot read CA file", "err", err)
			os.Exit(1)
		}
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		log.Error("CA file contained no usable certificate")
		os.Exit(1)
	}

	a := agent.New(agent.Config{
		HubAddr:  *hubAddr,
		Token:    *token,
		Version:  version,
		Platform: runtime.GOOS,
		TLS:      &tls.Config{RootCAs: pool, ServerName: *sni, MinVersion: tls.VersionTLS12},
		Logger:   log,
		Paused:   pauseFunc(*pauseF),
	})

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Info("peer agent starting", "hub", *hubAddr, "version", version, "platform", runtime.GOOS)
	go reportLoop(ctx, a, log)
	if err := a.Run(ctx); err != nil && ctx.Err() == nil {
		log.Error("agent stopped", "err", err)
		os.Exit(1)
	}
}

func pauseFunc(path string) func() bool {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	return func() bool {
		_, err := os.Stat(path)
		return err == nil
	}
}

func reportLoop(ctx context.Context, a *agent.Agent, log *slog.Logger) {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			log.Info("status", "shared_bytes_session", a.BytesToday())
		}
	}
}

func logOutput(path string) (io.Writer, error) {
	if strings.TrimSpace(path) == "" {
		return os.Stdout, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	flags := os.O_CREATE | os.O_WRONLY | os.O_APPEND
	if info, err := os.Stat(path); err == nil && info.Size() > maxLogBytes {
		flags = os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	}
	return os.OpenFile(path, flags, 0o600)
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func logLevel() slog.Level {
	if os.Getenv("LOG_LEVEL") == "debug" {
		return slog.LevelDebug
	}
	return slog.LevelInfo
}
