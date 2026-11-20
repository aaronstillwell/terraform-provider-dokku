package provider

import (
	"bytes"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/melbahja/goph"
	"golang.org/x/crypto/ssh"
)

func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(os.Stderr)
		log.SetFlags(log.LstdFlags)
	})
	return &buf
}

func loggedSSHLines(buf *bytes.Buffer) []string {
	var lines []string
	for _, line := range strings.Split(buf.String(), "\n") {
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

func TestDebugSSHWriterLogsEachLineWithLevelPrefix(t *testing.T) {
	logs := captureLogs(t)
	w := newDebugSSHWriter(nil)

	if _, err := w.Write([]byte("-----> Setting config vars\n-----> Restarting app movie-tickets\n")); err != nil {
		t.Fatal(err)
	}
	w.flush()

	got := loggedSSHLines(logs)
	want := []string{
		"[DEBUG] SSH: -----> Setting config vars",
		"[DEBUG] SSH: -----> Restarting app movie-tickets",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("logged lines:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestDebugSSHWriterSplitsCarriageReturns(t *testing.T) {
	logs := captureLogs(t)
	w := newDebugSSHWriter(nil)

	if _, err := w.Write([]byte("Waiting for 10s\rWaiting for 20s\n")); err != nil {
		t.Fatal(err)
	}
	w.flush()

	got := loggedSSHLines(logs)
	want := []string{
		"[DEBUG] SSH: Waiting for 10s",
		"[DEBUG] SSH: Waiting for 20s",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("logged lines:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestDebugSSHWriterRedactsSensitiveStrings(t *testing.T) {
	logs := captureLogs(t)
	w := newDebugSSHWriter([]string{"super-secret", ""})

	if _, err := w.Write([]byte("GEOIP_BASE_URL: super-secret\n")); err != nil {
		t.Fatal(err)
	}
	w.flush()

	got := loggedSSHLines(logs)
	if len(got) != 1 || got[0] != "[DEBUG] SSH: GEOIP_BASE_URL: *******" {
		t.Fatalf("logged lines = %#v", got)
	}
	if !strings.Contains(w.String(), "super-secret") {
		t.Fatalf("raw buffer should keep the original value, got %q", w.String())
	}
}

func TestDebugSSHWriterFlushesPartialLastLine(t *testing.T) {
	logs := captureLogs(t)
	w := newDebugSSHWriter(nil)

	if _, err := w.Write([]byte("no trailing newline")); err != nil {
		t.Fatal(err)
	}
	if lines := loggedSSHLines(logs); len(lines) != 0 {
		t.Fatalf("partial line logged before flush: %#v", lines)
	}

	w.flush()

	got := loggedSSHLines(logs)
	if len(got) != 1 || got[0] != "[DEBUG] SSH: no trailing newline" {
		t.Fatalf("logged lines = %#v", got)
	}
}

func TestDebugSSHWriterSkipsEmptyLines(t *testing.T) {
	logs := captureLogs(t)
	w := newDebugSSHWriter(nil)

	if _, err := w.Write([]byte("keep\n\n   \r\nkeep-too\n")); err != nil {
		t.Fatal(err)
	}
	w.flush()

	got := loggedSSHLines(logs)
	want := []string{
		"[DEBUG] SSH: keep",
		"[DEBUG] SSH: keep-too",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("logged lines:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestRedactSkipsEmptySecrets(t *testing.T) {
	got := redact("abc", []string{"", "b"})
	if got != "a*******c" {
		t.Fatalf("redact = %q", got)
	}
}

// testSSHClient performs a real handshake against a local, single-client server.
func testSSHClient(t *testing.T, config *ssh.ServerConfig, auth []ssh.AuthMethod, handle func(ssh.NewChannel)) *goph.Client {
	t.Helper()
	hostKey, err := ssh.ParsePrivateKey(generateEd25519PEM(t))
	if err != nil {
		t.Fatal(err)
	}
	config.AddHostKey(hostKey)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		conn.SetDeadline(time.Now().Add(10 * time.Second))
		server, channels, requests, err := ssh.NewServerConn(conn, config)
		if err != nil {
			t.Errorf("SSH server handshake: %v", err)
			return
		}
		defer server.Close()
		go ssh.DiscardRequests(requests)
		for channel := range channels {
			if handle == nil {
				channel.Reject(ssh.UnknownChannelType, "no sessions")
			} else {
				handle(channel)
			}
		}
	}()
	t.Cleanup(func() {
		listener.Close()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Error("SSH test server did not stop")
		}
	})
	client, err := ssh.Dial("tcp", listener.Addr().String(), &ssh.ClientConfig{
		User: "dokku", Auth: auth, HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("SSH client handshake: %v", err)
	}
	t.Cleanup(func() { client.Close() })
	return &goph.Client{Client: client}
}

func TestRunSuppressesSecretOutput(t *testing.T) {
	for _, cmd := range []string{
		"config:show app", "postgres:info db", "mysql:info db",
		"redis:info db", "clickhouse:info db",
	} {
		for _, status := range []uint32{0, 1} {
			t.Run(fmt.Sprintf("%s/status=%d", cmd, status), func(t *testing.T) {
				logs := captureLogs(t)
				output := "PASSWORD: super-secret\nDATABASE_URL: postgres://user:password@db\rpartial-secret"
				client := testSSHClient(t, &ssh.ServerConfig{NoClientAuth: true}, nil, func(newChannel ssh.NewChannel) {
					channel, requests, err := newChannel.Accept()
					if err != nil {
						t.Errorf("accept session: %v", err)
						return
					}
					defer channel.Close()
					for request := range requests {
						if request.Type != "exec" {
							request.Reply(false, nil)
							continue
						}
						request.Reply(true, nil)
						channel.Write([]byte(output))
						channel.Stderr().Write([]byte("\nstderr-secret\n"))
						channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{status}))
						return
					}
				})
				result := run(client, cmd)
				if result.stdout != output+"\nstderr-secret\n" {
					// Stdout and stderr may be interleaved, but both must be retained.
					if !strings.Contains(result.stdout, output) || !strings.Contains(result.stdout, "stderr-secret") {
						t.Fatalf("output missing from result: %q", result.stdout)
					}
				}
				if result.status != int(status) || (result.err != nil) != (status != 0) {
					t.Fatalf("unexpected command result: %#v", result)
				}
				for _, secret := range []string{"super-secret", "user:password", "partial-secret", "stderr-secret"} {
					if strings.Contains(logs.String(), secret) {
						t.Errorf("secret %q leaked in logs", secret)
					}
					if result.err != nil && strings.Contains(result.err.Error(), secret) {
						t.Errorf("secret %q leaked in error", secret)
					}
				}
				if !strings.Contains(logs.String(), "[DEBUG] SSH: "+cmd) {
					t.Error("command itself should still be logged")
				}
			})
		}
	}
}
