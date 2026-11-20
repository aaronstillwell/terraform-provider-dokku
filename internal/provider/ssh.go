package provider

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/melbahja/goph"
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
