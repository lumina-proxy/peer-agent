package agent

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/yamux"
	"github.com/lumina-proxy/peer-agent/internal/protocol"
)

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func hubSession(t *testing.T, a *Agent) *yamux.Session {
	t.Helper()
	hubSide, agentSide := net.Pipe()
	server, err := yamux.Server(agentSide, yamux.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	client, err := yamux.Client(hubSide, yamux.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for {
			stream, err := server.AcceptStream()
			if err != nil {
				return
			}
			go a.handleStream(stream)
		}
	}()
	t.Cleanup(func() {
		client.Close()
		server.Close()
	})
	return client
}

func requestTarget(t *testing.T, sess *yamux.Session, target string) (*yamux.Stream, *bufio.Reader, string) {
	t.Helper()
	stream, err := sess.OpenStream()
	if err != nil {
		t.Fatal(err)
	}
	if err := protocol.WriteLine(stream, target); err != nil {
		t.Fatal(err)
	}
	br := bufio.NewReader(stream)
	line, err := protocol.ReadLine(br)
	if err != nil {
		t.Fatalf("%s: %v", target, err)
	}
	return stream, br, line
}

func TestAgentRefusesUnsafeTargets(t *testing.T) {
	a := New(Config{Logger: quietLogger(), DialTimeout: 2 * time.Second})
	sess := hubSession(t, a)

	targets := []string{
		"127.0.0.1:80",
		"localhost:443",
		"10.0.0.5:443",
		"172.16.3.4:80",
		"192.168.1.1:80",
		"169.254.169.254:80",
		"100.64.0.1:443",
		"[::1]:443",
		"[fd00::1]:443",
		"example.com:25",
		"example.com:22",
		"example.com:8080",
		"not-a-target",
	}
	for _, target := range targets {
		stream, _, reply := requestTarget(t, sess, target)
		stream.Close()
		if !strings.HasPrefix(reply, "ERR") {
			t.Errorf("%s: agent replied %q, want a refusal", target, reply)
		}
	}
}

func TestAgentPipesAllowedTarget(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = io.Copy(conn, conn)
	}()

	var dialed string
	a := New(Config{
		Logger: quietLogger(),
		Dial: func(ctx context.Context, network, addr string) (net.Conn, error) {
			dialed = addr
			var d net.Dialer
			return d.DialContext(ctx, network, ln.Addr().String())
		},
	})
	sess := hubSession(t, a)

	stream, br, reply := requestTarget(t, sess, "example.com:443")
	defer stream.Close()
	if reply != "OK" {
		t.Fatalf("agent replied %q, want OK", reply)
	}
	if dialed != "example.com:443" {
		t.Fatalf("agent dialed %q, want example.com:443", dialed)
	}
	if _, err := io.WriteString(stream, "ping\n"); err != nil {
		t.Fatal(err)
	}
	echo, err := br.ReadString('\n')
	if err != nil || echo != "ping\n" {
		t.Fatalf("echo = %q, err %v", echo, err)
	}
}
