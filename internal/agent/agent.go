package agent

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"log/slog"
	"math/rand"
	"net"
	"sync/atomic"
	"time"

	"github.com/hashicorp/yamux"
	"github.com/lumina-proxy/peer-agent/internal/netguard"
	"github.com/lumina-proxy/peer-agent/internal/protocol"
)

type Config struct {
	HubAddr     string
	Token       string
	TLS         *tls.Config
	Version     string
	Platform    string
	DialTimeout time.Duration
	Logger      *slog.Logger

	Paused func() bool

	Dial func(ctx context.Context, network, addr string) (net.Conn, error)
}

type Agent struct {
	cfg        Config
	bytesToday atomic.Int64
}

func New(cfg Config) *Agent {
	if cfg.DialTimeout == 0 {
		cfg.DialTimeout = 20 * time.Second
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Platform == "" {
		cfg.Platform = "linux"
	}
	if cfg.Dial == nil {
		cfg.Dial = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return netguard.DialContext(ctx, network, addr, cfg.DialTimeout)
		}
	}
	return &Agent{cfg: cfg}
}

func (a *Agent) BytesToday() int64 { return a.bytesToday.Load() }

func (a *Agent) Run(ctx context.Context) error {
	backoff := time.Second
	const maxBackoff = 2 * time.Minute
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if a.cfg.Paused != nil && a.cfg.Paused() {
			if !sleepCtx(ctx, 5*time.Second) {
				return ctx.Err()
			}
			continue
		}

		err := a.session(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if errors.Is(err, errTokenRejected) {
			a.cfg.Logger.Error("hub rejected the device token; will retry less often", "err", err)
			if !sleepCtx(ctx, 5*time.Minute) {
				return ctx.Err()
			}
			continue
		}
		a.cfg.Logger.Warn("hub connection ended, reconnecting", "err", err, "backoff", backoff.String())
		if !sleepCtx(ctx, jitter(backoff)) {
			return ctx.Err()
		}
		if backoff < maxBackoff {
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

var errTokenRejected = errors.New("device token rejected")

func (a *Agent) session(ctx context.Context) error {
	dialer := &net.Dialer{Timeout: a.cfg.DialTimeout}
	raw, err := dialer.DialContext(ctx, "tcp", a.cfg.HubAddr)
	if err != nil {
		return err
	}
	conn := tls.Client(raw, a.cfg.TLS)
	if err := conn.HandshakeContext(ctx); err != nil {
		raw.Close()
		return err
	}

	reg := protocol.Registration{Token: a.cfg.Token, Version: a.cfg.Version, Platform: a.cfg.Platform}
	if err := protocol.WriteJSONLine(conn, reg); err != nil {
		conn.Close()
		return err
	}
	br := bufio.NewReader(conn)
	var ack protocol.RegistrationAck
	if err := protocol.ReadJSONLine(br, &ack); err != nil {
		conn.Close()
		return err
	}
	if !ack.OK {
		conn.Close()
		return errTokenRejected
	}
	a.cfg.Logger.Info("registered with hub", "country", ack.Country)

	session, err := yamux.Server(conn, yamux.DefaultConfig())
	if err != nil {
		conn.Close()
		return err
	}
	defer session.Close()

	go func() {
		<-ctx.Done()
		session.Close()
	}()

	for {
		stream, err := session.AcceptStream()
		if err != nil {
			return err
		}
		if a.cfg.Paused != nil && a.cfg.Paused() {
			stream.Close()
			return errors.New("paused")
		}
		go a.handleStream(stream)
	}
}

func (a *Agent) handleStream(stream *yamux.Stream) {
	defer stream.Close()
	br := bufio.NewReader(stream)
	target, err := protocol.ReadLine(br)
	if err != nil {
		return
	}
	if _, _, err := net.SplitHostPort(target); err != nil {
		_ = protocol.WriteLine(stream, "ERR bad target")
		return
	}

	dialCtx, cancel := context.WithTimeout(context.Background(), a.cfg.DialTimeout)
	upstream, err := a.cfg.Dial(dialCtx, "tcp", target)
	cancel()
	if err != nil {
		a.cfg.Logger.Debug("refused or failed to dial target", "target", target, "err", err)
		_ = protocol.WriteLine(stream, "ERR "+err.Error())
		return
	}
	defer upstream.Close()

	if err := protocol.WriteLine(stream, "OK"); err != nil {
		return
	}

	a.pipe(stream, br, upstream)
}

func (a *Agent) pipe(stream *yamux.Stream, buffered *bufio.Reader, upstream net.Conn) {
	done := make(chan struct{}, 2)
	go func() {
		n, _ := io.Copy(upstream, buffered)
		a.bytesToday.Add(n)
		if c, ok := upstream.(interface{ CloseWrite() error }); ok {
			c.CloseWrite()
		}
		done <- struct{}{}
	}()
	go func() {
		n, _ := io.Copy(stream, upstream)
		a.bytesToday.Add(n)
		stream.Close()
		done <- struct{}{}
	}()
	<-done
	<-done
}

func jitter(d time.Duration) time.Duration {
	return d/2 + time.Duration(rand.Int63n(int64(d)))
}

func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
