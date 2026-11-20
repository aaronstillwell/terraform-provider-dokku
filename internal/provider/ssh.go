package provider

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/melbahja/goph"
	"golang.org/x/crypto/ssh"
)

type SshOutput struct {
	stdout string
	// status code will be 0 if there is no error, otherwise
	// the status code extracted form the error
	status int
	err    error
}

// debugSSHWriter tees SSH stdout/stderr into a buffer and logs each complete
// line with a [DEBUG] prefix. Terraform/OpenTofu read provider logs line-by-line
// and drop lines that lack a level tag, so multi-line CombinedOutput dumps never
// show up under TF_LOG=DEBUG.
//
// \r is treated as a line break because dokku/docker health-check progress
// rewrites the same line with carriage returns.
//
// Writes are serialized: ssh.Session copies stdout and stderr concurrently.
type debugSSHWriter struct {
	mu             sync.Mutex
	raw            bytes.Buffer
	line           bytes.Buffer
	sensitive      []string
	suppressOutput bool
}

func newDebugSSHWriter(sensitive []string) *debugSSHWriter {
	return &debugSSHWriter{sensitive: sensitive}
}

func (w *debugSSHWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.raw.Write(p)
	if w.suppressOutput {
		return len(p), nil
	}
	for _, b := range p {
		if b == '\n' || b == '\r' {
			w.flushLineLocked()
			continue
		}
		w.line.WriteByte(b)
	}
	return len(p), nil
}

func (w *debugSSHWriter) flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.flushLineLocked()
}

func (w *debugSSHWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.raw.String()
}

func (w *debugSSHWriter) flushLineLocked() {
	if w.line.Len() == 0 {
		return
	}
	line := w.line.String()
	w.line.Reset()
	if strings.TrimSpace(line) == "" {
		return
	}
	log.Printf("[DEBUG] SSH: %s", redact(line, w.sensitive))
}

func redact(s string, sensitive []string) string {
	for _, toReplace := range sensitive {
		if toReplace == "" {
			continue
		}
		s = strings.Replace(s, toReplace, "*******", -1)
	}
	return s
}

// These responses contain secrets that are only known after the command runs.
// Keep the raw output for resource parsing, but never stream it to logs.
func sensitiveSSHOutput(cmd string) bool {
	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return false
	}
	return fields[0] == "config:show" ||
		strings.HasSuffix(fields[0], ":info") ||
		fields[0] == "logs:report" ||
		fields[0] == "logs:set" ||
		fields[0] == "logs:vector-logs"
}

// Run a command using the provided SSH client
//
// strings to be removed from logging can also be provided via `sensitiveStrings`
func run(client *goph.Client, cmd string, sensitiveStrings ...string) SshOutput {
	cmdSafe := redact(cmd, sensitiveStrings)
	log.Printf("[DEBUG] SSH: %s", cmdSafe)

	sess, err := client.NewSession()
	if err != nil {
		return SshOutput{
			err: errors.New(fmt.Sprintf("failed to create SSH session: %s", err)),
		}
	}
	defer sess.Close()

	w := newDebugSSHWriter(sensitiveStrings)
	w.suppressOutput = sensitiveSSHOutput(cmd)
	sess.Stdout = w
	sess.Stderr = w

	err = sess.Run(cmd)
	w.flush()

	stdout := redact(w.String(), sensitiveStrings)

	if err != nil {
		status := parseStatusCode(err.Error())
		log.Printf("[DEBUG] SSH: error status %d from %s", status, cmdSafe)
		errorOutput := stdout
		if w.suppressOutput {
			errorOutput = "command output suppressed because it may contain secrets"
		}
		return SshOutput{
			stdout: stdout,
			status: status,
			err:    errors.New(fmt.Sprintf("Error [%d]: %s", status, errorOutput)),
		}
	}

	return SshOutput{
		stdout: stdout,
		status: 0,
		err:    nil,
	}
}

func parseStatusCode(str string) int {
	re := regexp.MustCompile("^Process exited with status ([0-9]+)$")
	found := re.FindStringSubmatch(str)

	if found == nil {
		return 0
	}

	i, err := strconv.Atoi(found[1])

	if err != nil {
		return 0
	}

	return i
}

// runPostgresBackupCommand captures output without logging the command or stream.
// Backup authentication carries credentials in its arguments, and even info
// output may contain the database DSN. Never return remote failure output.
func runPostgresBackupCommand(ctx context.Context, client *goph.Client, command string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	sess, err := client.NewSession()
	if err != nil {
		return "", errors.New("failed to create backup SSH session")
	}
	defer sess.Close()
	var stdout bytes.Buffer
	sess.Stdout = &stdout
	sess.Stderr = io.Discard
	if err := sess.Start(command); err != nil {
		return "", errors.New("failed to start backup SSH command")
	}
	done := make(chan error, 1)
	go func() { done <- sess.Wait() }()
	select {
	case <-ctx.Done():
		_ = sess.Close()
		<-done
		return "", ctx.Err()
	case err := <-done:
		if err != nil {
			return "", errors.New("backup SSH command failed")
		}
		return stdout.String(), nil
	}
}

// runLogsCommand captures stdout and stderr without logging the command or
// streams. logs:report --format json, logs:set, and logs:vector-logs may
// contain plaintext vector sink DSNs with embedded credentials.
func runLogsCommand(ctx context.Context, client *goph.Client, command string) (string, string, int, error) {
	if err := ctx.Err(); err != nil {
		return "", "", 0, err
	}
	sess, err := client.NewSession()
	if err != nil {
		return "", "", 0, errors.New("failed to create logs SSH session")
	}
	defer sess.Close()
	var stdout, stderr bytes.Buffer
	sess.Stdout = &stdout
	sess.Stderr = &stderr
	if err := sess.Start(command); err != nil {
		return "", "", 0, errors.New("failed to start logs SSH command")
	}
	done := make(chan error, 1)
	go func() { done <- sess.Wait() }()
	select {
	case <-ctx.Done():
		_ = sess.Close()
		<-done
		return "", "", 0, ctx.Err()
	case err := <-done:
		if err != nil {
			status := 0
			var exitErr *ssh.ExitError
			if errors.As(err, &exitErr) {
				status = exitErr.ExitStatus()
			} else {
				status = parseStatusCode(err.Error())
			}
			if status > 0 {
				return stdout.String(), stderr.String(), status, fmt.Errorf("logs SSH command exited with status %d", status)
			}
			return stdout.String(), stderr.String(), 0, errors.New("logs SSH command failed")
		}
		return stdout.String(), stderr.String(), 0, nil
	}
}
