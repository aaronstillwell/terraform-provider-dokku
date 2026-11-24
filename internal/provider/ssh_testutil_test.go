package provider

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"net"
	"testing"
	"time"

	"github.com/melbahja/goph"
	"golang.org/x/crypto/ssh"
)

// Test helpers shared by the SSH and provider auth tests.

func generateEd25519PEM(t *testing.T) []byte {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate ed25519 key: %v", err)
	}
	block, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatalf("failed to marshal private key: %v", err)
	}
	return pem.EncodeToMemory(block)
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
