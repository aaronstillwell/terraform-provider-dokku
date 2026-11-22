package provider

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

func generateRSAPEM(t *testing.T) []byte {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}
	block := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(priv),
	}
	return pem.EncodeToMemory(block)
}

func TestProviderSchema_SSHCertIsOptional(t *testing.T) {
	p := Provider()
	certSchema, ok := p.Schema["ssh_cert"]
	if !ok {
		t.Fatal("ssh_cert schema missing")
	}
	if certSchema.Required {
		t.Errorf("expected ssh_cert.Required to be false, got true")
	}
	if !certSchema.Optional {
		t.Errorf("expected ssh_cert.Optional to be true, got false")
	}
}

func TestResolveDefaultSSHAuth_NoAgentNoKeys(t *testing.T) {
	emptyHome := t.TempDir()
	_, err := resolveDefaultSSHAuth("", emptyHome, "", "", "", 22)
	if err == nil {
		t.Fatal("expected error when no agent and no keys exist, got nil")
	}
	if !strings.Contains(err.Error(), "no default SSH keys found") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestResolveDefaultSSHAuth_DefaultEd25519Key(t *testing.T) {
	fakeHome := t.TempDir()
	sshDir := filepath.Join(fakeHome, ".ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		t.Fatal(err)
	}

	keyPEM := generateEd25519PEM(t)
	keyPath := filepath.Join(sshDir, "id_ed25519")
	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		t.Fatal(err)
	}

	auth, err := resolveDefaultSSHAuth("", fakeHome, "", "", "", 22)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(auth) != 1 {
		t.Fatalf("expected 1 auth method, got %d", len(auth))
	}
}

func TestResolveDefaultSSHAuth_DefaultRSAKey(t *testing.T) {
	fakeHome := t.TempDir()
	sshDir := filepath.Join(fakeHome, ".ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		t.Fatal(err)
	}

	keyPEM := generateRSAPEM(t)
	keyPath := filepath.Join(sshDir, "id_rsa")
	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		t.Fatal(err)
	}

	auth, err := resolveDefaultSSHAuth("", fakeHome, "", "", "", 22)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(auth) != 1 {
		t.Fatalf("expected 1 auth method, got %d", len(auth))
	}
}

func TestResolveDefaultSSHAuth_SSHAgent(t *testing.T) {
	sockDir, err := os.MkdirTemp("/tmp", "dokku-agent-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(sockDir)
	sockPath := filepath.Join(sockDir, "agent.sock")

	l, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	keyring := agent.NewKeyring()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := keyring.Add(agent.AddedKey{PrivateKey: priv}); err != nil {
		t.Fatal(err)
	}

	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go agent.ServeAgent(keyring, conn)
		}
	}()

	emptyHome := t.TempDir()
	auth, err := resolveDefaultSSHAuth(sockPath, emptyHome, "", "", "", 22)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(auth) != 1 {
		t.Fatalf("expected 1 auth method from agent, got %d", len(auth))
	}
}

func TestResolveDefaultSSHAuth_SSHAgentEmpty(t *testing.T) {
	sockDir, err := os.MkdirTemp("/tmp", "dokku-agent-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(sockDir)
	sockPath := filepath.Join(sockDir, "agent.sock")

	l, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	keyring := agent.NewKeyring()

	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go agent.ServeAgent(keyring, conn)
		}
	}()

	emptyHome := t.TempDir()
	_, err = resolveDefaultSSHAuth(sockPath, emptyHome, "", "", "", 22)
	if err == nil {
		t.Fatal("expected error when agent has no keys and ~/.ssh has no keys, got nil")
	}
	if !strings.Contains(err.Error(), "no default SSH keys found") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestResolveSSHAuth_ExplicitCertFile(t *testing.T) {
	keyPEM := generateEd25519PEM(t)
	tmpFile := filepath.Join(t.TempDir(), "test_key")
	if err := os.WriteFile(tmpFile, keyPEM, 0600); err != nil {
		t.Fatal(err)
	}

	auth, err := resolveSSHAuth(tmpFile, "", "", "", 22)
	if err != nil {
		t.Fatalf("unexpected error loading explicit cert file: %v", err)
	}
	if len(auth) != 1 {
		t.Fatalf("expected 1 auth method, got %d", len(auth))
	}
}

func TestResolveSSHAuth_InlineCert(t *testing.T) {
	keyPEM := generateEd25519PEM(t)

	auth, err := resolveSSHAuth(string(keyPEM), "", "", "", 22)
	if err != nil {
		t.Fatalf("unexpected error loading inline cert: %v", err)
	}
	if len(auth) != 1 {
		t.Fatalf("expected 1 auth method, got %d", len(auth))
	}
}

func TestResolveDefaultSSHAuth_UnauthorizedAgentFallsBackToDisk(t *testing.T) {
	// Use a short Unix socket path to stay within macOS socket path limits.
	sockDir, err := os.MkdirTemp("/tmp", "dokku-agent-test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(sockDir) })
	sockPath := filepath.Join(sockDir, "agent.sock")
	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	keyring := agent.NewKeyring()
	agentPublic, agentPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := keyring.Add(agent.AddedKey{PrivateKey: agentPrivate}); err != nil {
		t.Fatal(err)
	}
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		agent.ServeAgent(keyring, conn)
	}()
	fakeHome := t.TempDir()
	sshDir := filepath.Join(fakeHome, ".ssh")
	if err := os.Mkdir(sshDir, 0700); err != nil {
		t.Fatal(err)
	}
	keyPEM := generateEd25519PEM(t)
	if err := os.WriteFile(filepath.Join(sshDir, "id_ed25519"), keyPEM, 0600); err != nil {
		t.Fatal(err)
	}
	diskSigner, err := ssh.ParsePrivateKey(keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	agentKey, err := ssh.NewPublicKey(agentPublic)
	if err != nil {
		t.Fatal(err)
	}
	auth, err := resolveDefaultSSHAuth(sockPath, fakeHome, "", "", "", 22)
	if err != nil {
		t.Fatal(err)
	}
	var attempted [][]byte
	testSSHClient(t, &ssh.ServerConfig{
		PublicKeyCallback: func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			attempted = append(attempted, key.Marshal())
			if bytes.Equal(key.Marshal(), diskSigner.PublicKey().Marshal()) {
				return nil, nil
			}
			return nil, fmt.Errorf("unauthorized key")
		},
	}, auth, nil)
	if len(attempted) < 2 || !bytes.Equal(attempted[0], agentKey.Marshal()) || !bytes.Equal(attempted[1], diskSigner.PublicKey().Marshal()) {
		t.Fatal("expected rejected agent key followed by authorized disk key")
	}
}

func startTestSSHAgent(t *testing.T, sockDirPrefix string, count int) (string, []ssh.PublicKey) {
	t.Helper()
	sockDir, err := os.MkdirTemp("/tmp", sockDirPrefix)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(sockDir) })
	sockPath := filepath.Join(sockDir, "agent.sock")

	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })

	keyring := agent.NewKeyring()
	pubKeys := make([]ssh.PublicKey, 0, count)
	for i := 0; i < count; i++ {
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		if err := keyring.Add(agent.AddedKey{
			PrivateKey: priv,
			Comment:    fmt.Sprintf("key-%d", i),
		}); err != nil {
			t.Fatal(err)
		}
		sshPub, err := ssh.NewPublicKey(pub)
		if err != nil {
			t.Fatal(err)
		}
		pubKeys = append(pubKeys, sshPub)
	}

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go agent.ServeAgent(keyring, conn)
		}
	}()

	return sockPath, pubKeys
}

func writeSSHConfigFiles(t *testing.T, fakeHome string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		full := filepath.Join(fakeHome, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestResolveDefaultSSHAuth_PubIdentityFileAndIdentitiesOnlyWithNineAgentKeys(t *testing.T) {
	sockPath, agentKeys := startTestSSHAgent(t, "dokku-9keys-", 9)
	targetKey := agentKeys[7] // 8th key; would fail 6-attempt server cap without config discovery

	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	writeSSHConfigFiles(t, fakeHome, map[string]string{
		".ssh/1Password/dokku_key.pub": string(ssh.MarshalAuthorizedKey(targetKey)),
		".ssh/config": `
Host dokku.example
  IdentityFile ~/.ssh/1Password/dokku_key.pub
  IdentitiesOnly yes
`,
	})

	auth, err := resolveDefaultSSHAuth(sockPath, fakeHome, "", "dokku.example", "dokku", 22)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var attempted [][]byte
	testSSHClient(t, &ssh.ServerConfig{
		MaxAuthTries: 6,
		PublicKeyCallback: func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			attempted = append(attempted, key.Marshal())
			if bytes.Equal(key.Marshal(), targetKey.Marshal()) {
				return nil, nil
			}
			return nil, fmt.Errorf("unauthorized key")
		},
	}, auth, nil)

	if len(attempted) != 1 || !bytes.Equal(attempted[0], targetKey.Marshal()) {
		t.Fatalf("expected exactly 1 attempt with 8th agent key, got %d attempt(s)", len(attempted))
	}
}

func TestResolveDefaultSSHAuth_Include1PasswordMatchHostUser(t *testing.T) {
	sockPath, agentKeys := startTestSSHAgent(t, "dokku-1p-", 9)
	wrongUserKey := agentKeys[0]
	targetKey := agentKeys[7]

	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	writeSSHConfigFiles(t, fakeHome, map[string]string{
		".ssh/1Password/other.pub": string(ssh.MarshalAuthorizedKey(wrongUserKey)),
		".ssh/1Password/dokku.pub": string(ssh.MarshalAuthorizedKey(targetKey)),
		".ssh/1Password/config": `
Match Host dokku.example User otheruser
  IdentityFile ~/.ssh/1Password/other.pub
  IdentitiesOnly yes

Match Host dokku.example User dokku
  IdentityFile ~/.ssh/1Password/dokku.pub
  IdentitiesOnly yes

Match all
`,
		".ssh/config": `
Include ~/.ssh/1Password/config
Include ~/.ssh/nonexistent_optional_config
`,
	})

	auth, err := resolveDefaultSSHAuth(sockPath, fakeHome, "", "dokku.example", "dokku", 22)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var attempted [][]byte
	testSSHClient(t, &ssh.ServerConfig{
		MaxAuthTries: 6,
		PublicKeyCallback: func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			attempted = append(attempted, key.Marshal())
			if bytes.Equal(key.Marshal(), targetKey.Marshal()) {
				return nil, nil
			}
			return nil, fmt.Errorf("unauthorized key")
		},
	}, auth, nil)

	if len(attempted) != 1 || !bytes.Equal(attempted[0], targetKey.Marshal()) {
		t.Fatalf("expected only dokku user's matched 8th key, got %d attempt(s)", len(attempted))
	}
}

func TestResolveDefaultSSHAuth_IdentityFilePrioritizedWhenIdentitiesOnlyNo(t *testing.T) {
	sockPath, agentKeys := startTestSSHAgent(t, "dokku-order-", 9)
	prioritizedKey := agentKeys[7]

	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	writeSSHConfigFiles(t, fakeHome, map[string]string{
		".ssh/keys/dokku.pub": string(ssh.MarshalAuthorizedKey(prioritizedKey)),
		".ssh/config": `
Host dokku.example
  IdentityFile ~/.ssh/keys/dokku.pub
  IdentitiesOnly no
`,
	})

	auth, err := resolveDefaultSSHAuth(sockPath, fakeHome, "", "dokku.example", "dokku", 22)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify prioritized key is offered first (succeeds on try 1 under 6-attempt cap).
	var attempted [][]byte
	testSSHClient(t, &ssh.ServerConfig{
		MaxAuthTries: 6,
		PublicKeyCallback: func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			attempted = append(attempted, key.Marshal())
			if bytes.Equal(key.Marshal(), prioritizedKey.Marshal()) {
				return nil, nil
			}
			return nil, fmt.Errorf("unauthorized key")
		},
	}, auth, nil)
	if len(attempted) != 1 || !bytes.Equal(attempted[0], prioritizedKey.Marshal()) {
		t.Fatalf("expected prioritized key first, got %d attempt(s)", len(attempted))
	}

	// Verify remaining agent keys are still included when IdentitiesOnly is no.
	fallbackKey := agentKeys[2]
	attempted = nil
	testSSHClient(t, &ssh.ServerConfig{
		MaxAuthTries: 6,
		PublicKeyCallback: func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			attempted = append(attempted, key.Marshal())
			if bytes.Equal(key.Marshal(), fallbackKey.Marshal()) {
				return nil, nil
			}
			return nil, fmt.Errorf("unauthorized key")
		},
	}, auth, nil)
	if len(attempted) < 2 || !bytes.Equal(attempted[0], prioritizedKey.Marshal()) {
		t.Fatalf("expected prioritized key first followed by remaining agent keys, got %d attempt(s)", len(attempted))
	}
}

func TestResolveDefaultSSHAuth_IdentityAgentOverrideAndNone(t *testing.T) {
	envSock, _ := startTestSSHAgent(t, "dokku-env-", 1)
	overrideSock, overrideKeys := startTestSSHAgent(t, "dokku agent ", 1)

	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	writeSSHConfigFiles(t, fakeHome, map[string]string{
		".ssh/config": fmt.Sprintf(`
Host override.example
  IdentityAgent "%s"

Host none.example
  IdentityAgent none
`, overrideSock),
	})

	auth, err := resolveDefaultSSHAuth(envSock, fakeHome, "", "override.example", "dokku", 22)
	if err != nil {
		t.Fatalf("unexpected error for IdentityAgent override: %v", err)
	}
	var attempted [][]byte
	testSSHClient(t, &ssh.ServerConfig{
		PublicKeyCallback: func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			attempted = append(attempted, key.Marshal())
			if bytes.Equal(key.Marshal(), overrideKeys[0].Marshal()) {
				return nil, nil
			}
			return nil, fmt.Errorf("unauthorized key")
		},
	}, auth, nil)
	if len(attempted) != 1 || !bytes.Equal(attempted[0], overrideKeys[0].Marshal()) {
		t.Fatalf("expected key from overridden IdentityAgent socket")
	}

	// IdentityAgent none should skip envSock and fail if no disk keys exist.
	_, err = resolveDefaultSSHAuth(envSock, fakeHome, "", "none.example", "dokku", 22)
	if err == nil || !strings.Contains(err.Error(), "no default SSH keys found") {
		t.Fatalf("expected no default SSH keys error with IdentityAgent none, got %v", err)
	}
}

func TestResolveDefaultSSHAuth_IdentityFileTokensAndNoneAndMissing(t *testing.T) {
	sockPath, agentKeys := startTestSSHAgent(t, "dokku-tok-", 2)

	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	diskPEM := generateEd25519PEM(t)
	diskSigner, err := ssh.ParsePrivateKey(diskPEM)
	if err != nil {
		t.Fatal(err)
	}

	writeSSHConfigFiles(t, fakeHome, map[string]string{
		".ssh/id_ed25519":                    string(diskPEM),
		".ssh/tokens/dokku-tok.example.pub":  string(ssh.MarshalAuthorizedKey(agentKeys[1])),
		".ssh/config": `
Host tok.example
  IdentityFile %d/.ssh/tokens/%r-%h.pub
  IdentitiesOnly yes

Host noneid.example
  IdentityFile none
  IdentitiesOnly yes

Host missing.example
  IdentityFile ~/.ssh/does_not_exist.pub
  IdentitiesOnly yes
`,
	})

	// Token expansion (%d, %r, %h)
	auth, err := resolveDefaultSSHAuth(sockPath, fakeHome, "", "tok.example", "dokku", 22)
	if err != nil {
		t.Fatalf("unexpected error for token IdentityFile: %v", err)
	}
	var attempted [][]byte
	testSSHClient(t, &ssh.ServerConfig{
		PublicKeyCallback: func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			attempted = append(attempted, key.Marshal())
			if bytes.Equal(key.Marshal(), agentKeys[1].Marshal()) {
				return nil, nil
			}
			return nil, fmt.Errorf("unauthorized key")
		},
	}, auth, nil)
	if len(attempted) != 1 || !bytes.Equal(attempted[0], agentKeys[1].Marshal()) {
		t.Fatalf("expected token-expanded key only, got %d attempt(s)", len(attempted))
	}

	// IdentityFile none + IdentitiesOnly yes suppresses ~/.ssh/id_ed25519 and agent keys.
	_, err = resolveDefaultSSHAuth(sockPath, fakeHome, "", "noneid.example", "dokku", 22)
	if err == nil || !strings.Contains(err.Error(), "no default SSH keys found") {
		t.Fatalf("expected error with IdentityFile none + IdentitiesOnly yes, got %v", err)
	}

	// Missing IdentityFile + IdentitiesOnly yes does not fall back to ~/.ssh/id_ed25519 or agent.
	_, err = resolveDefaultSSHAuth(sockPath, fakeHome, "", "missing.example", "dokku", 22)
	if err == nil || !strings.Contains(err.Error(), "no default SSH keys found") {
		t.Fatalf("expected error with missing IdentityFile + IdentitiesOnly yes, got %v", err)
	}

	// Unmatched host with no IdentityFile uses agent + ~/.ssh/id_ed25519 fallback.
	auth, err = resolveDefaultSSHAuth(sockPath, fakeHome, "", "other.example", "dokku", 22)
	if err != nil {
		t.Fatalf("unexpected error for unmatched host: %v", err)
	}
	attempted = nil
	testSSHClient(t, &ssh.ServerConfig{
		PublicKeyCallback: func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			attempted = append(attempted, key.Marshal())
			if bytes.Equal(key.Marshal(), diskSigner.PublicKey().Marshal()) {
				return nil, nil
			}
			return nil, fmt.Errorf("unauthorized key")
		},
	}, auth, nil)
	if len(attempted) != 3 {
		t.Fatalf("expected 2 agent keys + 1 disk key = 3 attempts, got %d", len(attempted))
	}
}

func TestResolveSSHAuth_ExplicitCertIgnoresSSHConfig(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	writeSSHConfigFiles(t, fakeHome, map[string]string{
		".ssh/config": `
Host dokku.example
  IdentityFile ~/.ssh/nonexistent.pub
  IdentitiesOnly yes
  IdentityAgent none
`,
	})

	keyPEM := generateEd25519PEM(t)
	keyPath := filepath.Join(fakeHome, "explicit_key")
	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		t.Fatal(err)
	}

	auth, err := resolveSSHAuth(keyPath, "", "dokku.example", "dokku", 22)
	if err != nil {
		t.Fatalf("expected explicit ssh_cert to bypass ~/.ssh/config, got %v", err)
	}
	if len(auth) != 1 {
		t.Fatalf("expected 1 auth method, got %d", len(auth))
	}
}
