package provider

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"golang.org/x/crypto/ssh"
)

// backupFakeSSH provides a local SSH protocol endpoint. It emulates only the
// Dokku commands needed by this test and never executes commands on the host.
type backupFakeSSH struct {
	mu                                                                                                              sync.Mutex
	listener                                                                                                        net.Listener
	authenticated                                                                                                   bool
	schedule, bucket, region, signature, endpoint, fingerprint, accessFingerprint, secretFingerprint, currentSecret string
	failInfo, failDeauth, failAuthPre, failAuthPartial, failAuthPost, failSchedulePre, failSchedulePost             bool
	commands                                                                                                        []string
	injectScheduleOnAuth, cronSchedule                                                                              string
}

func newBackupFakeSSH(t *testing.T) *backupFakeSSH {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}
	config := &ssh.ServerConfig{NoClientAuth: true}
	config.AddHostKey(signer)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &backupFakeSSH{listener: listener}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				session, channels, requests, err := ssh.NewServerConn(conn, config)
				if err != nil {
					return
				}
				defer session.Close()
				go ssh.DiscardRequests(requests)
				for ch := range channels {
					go func(ch ssh.NewChannel) {
						channel, requests, err := ch.Accept()
						if err != nil {
							return
						}
						defer channel.Close()
						for req := range requests {
							if req.Type != "exec" {
								_ = req.Reply(false, nil)
								continue
							}
							var payload struct{ Command string }
							if err := ssh.Unmarshal(req.Payload, &payload); err != nil {
								_ = req.Reply(false, nil)
								continue
							}
							_ = req.Reply(true, nil)
							output, status := server.execute(payload.Command)
							_, _ = io.WriteString(channel, output)
							_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{status}))
							return
						}
					}(ch)
				}
			}()
		}
	}()
	t.Cleanup(func() { _ = listener.Close() })
	return server
}

// splitDokkuCommand recognizes shell-escaped words without running a shell.
func splitDokkuCommand(command string) ([]string, error) {
	var words []string
	var word strings.Builder
	var quote rune
	escaped, started := false, false
	for _, r := range command {
		if escaped {
			word.WriteRune(r)
			escaped = false
			started = true
			continue
		}
		if r == '\\' && quote != '\'' {
			escaped = true
			started = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				word.WriteRune(r)
			}
			started = true
			continue
		}
		switch r {
		case '\'', '"':
			quote = r
			started = true
		case ' ', '\t', '\n':
			if started {
				words = append(words, word.String())
				word.Reset()
				started = false
			}
		default:
			word.WriteRune(r)
			started = true
		}
	}
	if escaped || quote != 0 {
		return nil, fmt.Errorf("bad command quoting")
	}
	if started {
		words = append(words, word.String())
	}
	return words, nil
}

func (s *backupFakeSSH) execute(command string) (string, uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.commands = append(s.commands, command)
	words, err := splitDokkuCommand(command)
	if err != nil || len(words) == 0 {
		return "invalid command", 1
	}
	if len(words) > 1 && words[1] == "--" {
		words = append(words[:1], words[2:]...)
	}
	switch words[0] {
	case "version":
		return "dokku version 0.35.20\n", 0
	case "postgres:info":
		if s.failInfo {
			return "failed to inspect postgres", 1
		}
		schedules := ""
		if s.schedule != "" {
			schedules = fmt.Sprintf("default|%s|%s|false", s.schedule, s.bucket)
		}
		return fmt.Sprintf("=====> db postgres service information\nBackup schedule: %s\nBackup schedules: %s\nBackup bucket: %s\nBackup authenticated: %t\nBackup auth fingerprint: %s\nBackup access key id fingerprint: %s\nBackup secret access key fingerprint: %s\nBackup default region: %s\nBackup signature version: %s\nBackup endpoint url: %s\nBackup use iam: false\n", s.schedule, schedules, s.bucket, s.authenticated, s.fingerprint, s.accessFingerprint, s.secretFingerprint, s.region, s.signature, s.endpoint), 0
	case "postgres:backup-auth":
		if s.failAuthPre {
			s.failAuthPre = false
			return "simulated pre-auth failure: -BACKUP_SECRET_SENTINEL_728j", 1
		}
		if len(words) != 7 {
			return "bad auth arity", 1
		}
		if s.failAuthPartial {
			s.failAuthPartial = false
			s.accessFingerprint = fmt.Sprintf("%x", sha256.Sum256([]byte(words[2])))
			if s.currentSecret != "" {
				s.authenticated = true
				s.fingerprint = postgresBackupFingerprint(words[2], s.currentSecret)
			} else {
				s.authenticated = false
				s.fingerprint = ""
			}
			return "simulated partial auth failure: -BACKUP_SECRET_SENTINEL_728j", 1
		}
		s.authenticated, s.region, s.signature, s.endpoint = true, words[4], words[5], words[6]
		s.currentSecret = words[3]
		s.fingerprint = postgresBackupFingerprint(words[2], words[3])
		s.accessFingerprint = fmt.Sprintf("%x", sha256.Sum256([]byte(words[2])))
		s.secretFingerprint = fmt.Sprintf("%x", sha256.Sum256([]byte(words[3])))
		if s.injectScheduleOnAuth != "" {
			s.schedule = s.injectScheduleOnAuth
			s.injectScheduleOnAuth = ""
		}
		if s.failAuthPost {
			s.failAuthPost = false
			return "simulated auth failure: -BACKUP_SECRET_SENTINEL_728j", 1
		}
		return "authenticated\n", 0
	case "postgres:backup-schedule":
		if s.failSchedulePre {
			s.failSchedulePre = false
			return "simulated pre-schedule failure: -BACKUP_SECRET_SENTINEL_728j", 1
		}
		if len(words) != 4 {
			return "bad schedule arity", 1
		}
		s.schedule, s.bucket = words[2], words[3]
		if s.failSchedulePost {
			s.failSchedulePost = false
			return "simulated schedule failure: -BACKUP_SECRET_SENTINEL_728j", 1
		}
		s.cronSchedule = words[2]
		return "scheduled\n", 0
	case "postgres:backup-unschedule":
		s.schedule, s.bucket, s.cronSchedule = "", "", ""
		return "unscheduled\n", 0
	case "postgres:backup-deauth":
		if s.failDeauth {
			return "simulated deauth failure: -BACKUP_SECRET_SENTINEL_728j", 1
		}
		s.authenticated = false
		s.region, s.signature, s.endpoint, s.fingerprint, s.accessFingerprint, s.secretFingerprint, s.currentSecret = "", "", "", "", "", "", ""
		return "deauthenticated\n", 0
	default:
		return "unexpected command", 1
	}
}

func (s *backupFakeSSH) snapshot() (string, string, bool, []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.schedule, s.bucket, s.authenticated, append([]string(nil), s.commands...)
}

// TestBackupWriteOnlyCLI uses actual Terraform/OpenTofu and provider RPCs. It
// requires local CLIs but no external service, provider registry, or cloud keys.
func TestBackupWriteOnlyCLI(t *testing.T) {
	tools := []string{"terraform", "tofu"}
	if custom := os.Getenv("DOKKU_BACKUP_TEST_CLIS"); custom != "" {
		tools = filepath.SplitList(custom)
	}
	for _, tool := range tools {
		t.Run(filepath.Base(tool), func(t *testing.T) {
			cli, err := exec.LookPath(tool)
			if err != nil {
				if os.Getenv("DOKKU_BACKUP_TEST_CLIS") != "" {
					t.Fatalf("requested CLI %s: %v", tool, err)
				}
				t.Skipf("%s not installed", tool)
			}
			testBackupWriteOnlyCLI(t, cli, false)
		})
	}
}

func TestBackupWriteOnlyUnsupportedCLI(t *testing.T) {
	custom := os.Getenv("DOKKU_BACKUP_TEST_OLD_CLIS")
	if custom == "" {
		t.Skip("set DOKKU_BACKUP_TEST_OLD_CLIS to test unsupported CLI versions")
	}
	for _, tool := range filepath.SplitList(custom) {
		t.Run(filepath.Base(tool), func(t *testing.T) {
			cli, err := exec.LookPath(tool)
			if err != nil {
				t.Fatalf("requested CLI %s: %v", tool, err)
			}
			testBackupWriteOnlyCLI(t, cli, true)
		})
	}
}

func testBackupWriteOnlyCLI(t *testing.T, cli string, unsupported bool) {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	home := filepath.Join(dir, "home")
	for _, p := range []string{binDir, filepath.Join(home, ".ssh")} {
		if err := os.MkdirAll(p, 0700); err != nil {
			t.Fatal(err)
		}
	}
	build := exec.Command("go", "build", "-o", filepath.Join(binDir, "terraform-provider-dokku"), ".")
	build.Dir = root
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build provider: %v\n%s", err, output)
	}
	configFile := filepath.Join(dir, "terraformrc")
	cliConfig := fmt.Sprintf("provider_installation {\n  dev_overrides {\n    \"aaronstillwell/dokku\" = %s\n  }\n  direct {}\n}\n", strconv.Quote(binDir))
	if err := os.WriteFile(configFile, []byte(cliConfig), 0600); err != nil {
		t.Fatal(err)
	}
	key := filepath.Join(dir, "id_ed25519")
	if err := os.WriteFile(key, generateEd25519PEM(t), 0600); err != nil {
		t.Fatal(err)
	}
	server := newBackupFakeSSH(t)
	port := server.listener.Addr().(*net.TCPAddr).Port
	const access = "-BACKUP_ACCESS_SENTINEL_468g"
	const secret = "-BACKUP_SECRET_SENTINEL_728j"
	const rotatedSecret = "BACKUP_ROTATED_SENTINEL_915k"
	const rotatedAgain = "BACKUP_ROTATED_AGAIN_SENTINEL_347r"
	logPath := filepath.Join(dir, "debug.log")
	var env []string
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "TF_") || strings.HasPrefix(entry, "DOKKU_") || strings.HasPrefix(entry, "HOME=") {
			continue
		}
		env = append(env, entry)
	}
	env = append(env, "HOME="+home, "TF_CLI_CONFIG_FILE="+configFile, "TF_IN_AUTOMATION=1", "TF_LOG=DEBUG", "TF_LOG_PATH="+logPath, "TF_VAR_access="+access, "TF_VAR_secret="+secret)
	run := func(wantSuccess bool, args ...string) []byte {
		t.Helper()
		cmd := exec.Command(cli, args...)
		cmd.Dir, cmd.Env = dir, env
		out, err := cmd.CombinedOutput()
		if (err == nil) != wantSuccess {
			t.Fatalf("%s %v: %v\n%s", cli, args, err, out)
		}
		return out
	}
	writeConfig := func(revision int, schedule string) {
		t.Helper()
		config := fmt.Sprintf(`terraform {
  required_providers { dokku = { source = "aaronstillwell/dokku" } }
}
provider "dokku" {
  ssh_host = "127.0.0.1"
  ssh_user = "dokku"
  ssh_port = %d
  ssh_cert = %s
  skip_known_hosts_check = true
}
variable "access" {
  type = string
  sensitive = true
  ephemeral = true
}
variable "secret" {
  type = string
  sensitive = true
  ephemeral = true
}
resource "dokku_postgres_backup" "test" {
  service_name = "test-db"
  bucket = "backup-test"
  schedule = %s
  endpoint_url = "https://minio.example.test"
  region = "us-east-1"
  signature_version = "s3v4"
  access_key_id_wo = var.access
  secret_access_key_wo = var.secret
  credentials_version = %d
}
`, port, strconv.Quote(key), strconv.Quote(schedule), revision)
		if unsupported {
			config = strings.ReplaceAll(config, "  ephemeral = true\n", "")
		}
		if err := os.WriteFile(filepath.Join(dir, "main.tf"), []byte(config), 0600); err != nil {
			t.Fatal(err)
		}
	}
	checkNoSecret := func(label string, output []byte) {
		t.Helper()
		for _, value := range []string{access, secret, rotatedSecret, rotatedAgain} {
			if bytes.Contains(output, []byte(value)) {
				t.Fatalf("%s contains credential sentinel", label)
			}
		}
	}
	writeConfig(1, "0 3 * * *")
	if unsupported {
		output := run(false, "plan", "-input=false", "-out=planfile", "-no-color")
		checkNoSecret("unsupported CLI diagnostic", output)
		lower := strings.ToLower(string(output))
		if !strings.Contains(lower, "write-only") && !strings.Contains(lower, "write only") {
			t.Fatalf("missing write-only compatibility diagnostic: %s", output)
		}
		for _, file := range []string{"planfile", "terraform.tfstate"} {
			if data, err := os.ReadFile(filepath.Join(dir, file)); err == nil {
				checkNoSecret(file, data)
			}
		}
		return
	}
	run(true, "plan", "-input=false", "-out=planfile", "-no-color")
	checkNoSecret("saved plan JSON", run(true, "show", "-json", "planfile"))
	rawPlan, err := os.ReadFile(filepath.Join(dir, "planfile"))
	if err != nil {
		t.Fatal(err)
	}
	checkNoSecret("saved plan bytes", rawPlan)
	server.mu.Lock()
	server.failAuthPre = true
	server.mu.Unlock()
	failure := run(false, "apply", "-input=false", "-auto-approve", "-no-color", "planfile")
	checkNoSecret("pre-auth failure", failure)
	server.mu.Lock()
	server.failAuthPartial = true
	server.failDeauth = true
	server.mu.Unlock()
	failure = run(false, "apply", "-input=false", "-auto-approve", "-no-color")
	checkNoSecret("partial-auth failure", failure)
	if !bytes.Contains(run(true, "state", "list"), []byte("dokku_postgres_backup.test")) {
		t.Fatal("failed cleanup lost pending resource state")
	}
	server.mu.Lock()
	server.failDeauth = false
	server.failAuthPost = true
	server.mu.Unlock()
	failure = run(false, "apply", "-input=false", "-auto-approve", "-no-color")
	checkNoSecret("post-auth failure", failure)
	server.mu.Lock()
	server.failSchedulePre = true
	server.mu.Unlock()
	failure = run(false, "apply", "-input=false", "-auto-approve", "-no-color")
	checkNoSecret("pre-schedule failure", failure)
	server.mu.Lock()
	server.failSchedulePost = true
	server.mu.Unlock()
	failure = run(false, "apply", "-input=false", "-auto-approve", "-no-color")
	checkNoSecret("post-schedule failure", failure)
	run(true, "apply", "-input=false", "-auto-approve", "-no-color")
	state := run(true, "show", "-json")
	checkNoSecret("state JSON", state)
	var decoded map[string]interface{}
	if err := json.Unmarshal(state, &decoded); err != nil {
		t.Fatal(err)
	}
	schedule, bucket, authenticated, commands := server.snapshot()
	if schedule != "0 3 * * *" || bucket != "backup-test" || !authenticated {
		t.Fatalf("unexpected created schedule: %q %q %t", schedule, bucket, authenticated)
	}
	server.mu.Lock()
	if server.cronSchedule != "0 3 * * *" {
		server.mu.Unlock()
		t.Fatal("create retry left cron missing after partial schedule failure")
	}
	server.mu.Unlock()
	values := decoded["values"].(map[string]interface{})["root_module"].(map[string]interface{})["resources"].([]interface{})[0].(map[string]interface{})["values"].(map[string]interface{})
	for _, field := range []string{"access_key_id_wo", "secret_access_key_wo"} {
		if value := values[field]; value != nil {
			t.Fatalf("write-only field %s present in state", field)
		}
	}
	for i := range env {
		if strings.HasPrefix(env[i], "TF_VAR_secret=") {
			env[i] = "TF_VAR_secret=" + rotatedSecret
		}
	}
	plan := run(true, "plan", "-input=false", "-no-color")
	if !bytes.Contains(plan, []byte("No changes")) {
		t.Fatalf("write-only value unexpectedly caused a diff: %s", plan)
	}
	writeConfig(2, "15 4 * * *")
	server.mu.Lock()
	server.failAuthPre = true
	server.mu.Unlock()
	failure = run(false, "apply", "-input=false", "-auto-approve", "-no-color")
	checkNoSecret("pre-update auth failure", failure)
	server.mu.Lock()
	server.failAuthPartial = true
	server.mu.Unlock()
	failure = run(false, "apply", "-input=false", "-auto-approve", "-no-color")
	checkNoSecret("partial-update auth failure", failure)
	server.mu.Lock()
	server.failAuthPost = true
	server.mu.Unlock()
	failure = run(false, "apply", "-input=false", "-auto-approve", "-no-color")
	checkNoSecret("post-update auth failure", failure)
	server.mu.Lock()
	server.failSchedulePre = true
	server.mu.Unlock()
	failure = run(false, "apply", "-input=false", "-auto-approve", "-no-color")
	checkNoSecret("pre-update schedule failure", failure)
	server.mu.Lock()
	server.failSchedulePost = true
	server.mu.Unlock()
	failure = run(false, "apply", "-input=false", "-auto-approve", "-no-color")
	checkNoSecret("post-update schedule failure", failure)
	run(true, "apply", "-input=false", "-auto-approve", "-no-color")
	schedule, _, _, commands = server.snapshot()
	if schedule != "15 4 * * *" {
		t.Fatalf("schedule not updated: %q", schedule)
	}
	server.mu.Lock()
	if server.cronSchedule != "15 4 * * *" {
		server.mu.Unlock()
		t.Fatal("update retry left cron stale after partial schedule failure")
	}
	server.mu.Unlock()
	server.mu.Lock()
	if server.fingerprint != postgresBackupFingerprint(access, rotatedSecret) {
		server.mu.Unlock()
		t.Fatal("credential revision did not rotate the secret")
	}
	server.mu.Unlock()
	for i := range env {
		if strings.HasPrefix(env[i], "TF_VAR_secret=") {
			env[i] = "TF_VAR_secret=" + rotatedAgain
		}
	}
	writeConfig(3, "15 4 * * *")
	server.mu.Lock()
	server.injectScheduleOnAuth = "30 5 * * *"
	server.mu.Unlock()
	run(true, "apply", "-input=false", "-auto-approve", "-no-color")
	var afterAuth map[string]interface{}
	if err := json.Unmarshal(run(true, "show", "-json"), &afterAuth); err != nil {
		t.Fatal(err)
	}
	attrs := afterAuth["values"].(map[string]interface{})["root_module"].(map[string]interface{})["resources"].([]interface{})[0].(map[string]interface{})["values"].(map[string]interface{})
	if attrs["owned_schedule"] != "15 4 * * *" {
		t.Fatalf("credential update adopted concurrent schedule: %v", attrs["owned_schedule"])
	}
	server.mu.Lock()
	server.schedule = "10 5 * * *"
	server.mu.Unlock()
	drift := run(true, "plan", "-input=false", "-no-color")
	if !bytes.Contains(drift, []byte("15 4 * * *")) {
		t.Fatalf("schedule drift was not planned for correction: %s", drift)
	}
	run(true, "apply", "-input=false", "-auto-approve", "-no-color")
	schedule, _, _, commands = server.snapshot()
	if schedule != "15 4 * * *" {
		t.Fatalf("schedule drift not corrected: %s", schedule)
	}
	server.mu.Lock()
	originalFingerprint := server.fingerprint
	server.fingerprint = "externally-changed-fingerprint"
	server.schedule = "10 5 * * *"
	server.mu.Unlock()
	conflict := run(true, "apply", "-input=false", "-auto-approve", "-no-color")
	checkNoSecret("fingerprint conflict", conflict)
	if !bytes.Contains(conflict, []byte("Backup authentication changed outside Terraform")) {
		t.Fatal("missing external credential drift warning")
	}
	server.mu.Lock()
	if server.fingerprint != "externally-changed-fingerprint" || server.schedule != "15 4 * * *" {
		server.mu.Unlock()
		t.Fatal("schedule update modified external credentials or failed to reconcile schedule")
	}
	server.fingerprint = originalFingerprint
	server.mu.Unlock()
	checkState := run(true, "show", "-json")
	if !bytes.Contains(checkState, []byte(originalFingerprint)) || bytes.Contains(checkState, []byte("externally-changed-fingerprint")) {
		t.Fatal("schedule-only update adopted externally changed fingerprint")
	}

	run(true, "state", "rm", "dokku_postgres_backup.test")
	run(true, "import", "-input=false", "-no-color", "dokku_postgres_backup.test", "test-db")
	if !bytes.Contains(run(true, "state", "list"), []byte("dokku_postgres_backup.test")) {
		t.Fatal("import lost backup resource")
	}
	writeConfig(1, "15 4 * * *")
	run(true, "plan", "-input=false", "-detailed-exitcode", "-no-color")
	for _, command := range commands {
		if strings.HasPrefix(command, "postgres:create") || strings.HasPrefix(command, "postgres:destroy") || strings.HasPrefix(command, "postgres:clone") {
			t.Fatal("backup update changed the database")
		}
	}
	server.mu.Lock()
	server.failInfo = true
	server.mu.Unlock()
	run(false, "plan", "-input=false", "-no-color")
	if !bytes.Contains(run(true, "state", "list"), []byte("dokku_postgres_backup.test")) {
		t.Fatal("read failure removed resource from state")
	}
	server.mu.Lock()
	server.failInfo = false
	server.mu.Unlock()
	server.mu.Lock()
	server.failDeauth = true
	server.mu.Unlock()
	failure = run(false, "destroy", "-input=false", "-auto-approve", "-no-color")
	checkNoSecret("partial destroy failure", failure)
	server.mu.Lock()
	server.failDeauth = false
	server.mu.Unlock()
	run(true, "destroy", "-input=false", "-auto-approve", "-no-color")
	schedule, _, authenticated, _ = server.snapshot()
	if schedule != "" || authenticated {
		t.Fatal("destroy did not clear schedule and credentials")
	}
	server.mu.Lock()
	server.accessFingerprint = "foreign-partial-access"
	before := len(server.commands)
	server.mu.Unlock()
	conflict = run(false, "apply", "-input=false", "-auto-approve", "-no-color")
	checkNoSecret("partial-existing rejection", conflict)
	server.mu.Lock()
	for _, command := range server.commands[before:] {
		if strings.HasPrefix(command, "postgres:backup-") {
			server.mu.Unlock()
			t.Fatal("create modified externally owned partial credentials")
		}
	}
	if server.accessFingerprint != "foreign-partial-access" || server.secretFingerprint != "" {
		server.mu.Unlock()
		t.Fatal("create modified existing partial credentials")
	}
	server.schedule, server.bucket, server.authenticated, server.fingerprint, server.accessFingerprint, server.secretFingerprint = "0 2 * * *", "existing-bucket", true, "existing-fingerprint", "existing-access", "existing-secret"
	before = len(server.commands)
	server.mu.Unlock()
	conflict = run(false, "apply", "-input=false", "-auto-approve", "-no-color")
	checkNoSecret("create-existing rejection", conflict)
	server.mu.Lock()
	for _, command := range server.commands[before:] {
		if strings.HasPrefix(command, "postgres:backup-") {
			server.mu.Unlock()
			t.Fatal("create overwrote an existing backup")
		}
	}
	if server.schedule != "0 2 * * *" || server.bucket != "existing-bucket" || server.fingerprint != "existing-fingerprint" {
		server.mu.Unlock()
		t.Fatal("create changed existing configuration")
	}
	server.mu.Unlock()
	stateBytes, err := os.ReadFile(filepath.Join(dir, "terraform.tfstate"))
	if err != nil {
		t.Fatal(err)
	}
	checkNoSecret("raw state", stateBytes)
	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	checkNoSecret("debug log", logData)
}
