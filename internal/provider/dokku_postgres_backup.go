package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"al.essio.dev/pkg/shellescape"
	"github.com/melbahja/goph"
)

// postgresBackupRunner never exposes a command or its output in diagnostics.
// The boolean marks operations whose arguments contain credentials.
type postgresBackupRunner func(context.Context, string, bool) (string, error)

type postgresBackupState struct {
	ServiceExists        bool
	Scheduled            bool
	Schedule             string
	Bucket               string
	Region               string
	SignatureVersion     string
	Endpoint             string
	AuthFingerprint      string
	AccessKeyFingerprint string
	SecretKeyFingerprint string
	AuthPartial          bool
	Authenticated        bool
	UseIAM               bool
}

func newPostgresBackupRunner(client *goph.Client) postgresBackupRunner {
	return func(ctx context.Context, command string, sensitive bool) (string, error) {
		return runPostgresBackupCommand(ctx, client, command)
	}
}

func postgresBackupCommand(verb string, args ...string) (string, error) {
	parts := []string{"postgres:" + verb}
	if len(args) != 0 {
		parts = append(parts, "--")
	}
	for _, arg := range args {
		if strings.ContainsRune(arg, '\x00') {
			return "", errors.New("backup command argument contains a NUL byte")
		}
		parts = append(parts, shellescape.Quote(arg))
	}
	return strings.Join(parts, " "), nil
}

func runBackupOperation(ctx context.Context, run postgresBackupRunner, sensitive bool, verb string, args ...string) (string, error) {
	command, err := postgresBackupCommand(verb, args...)
	if err != nil {
		return "", err
	}
	output, err := run(ctx, command, sensitive)
	if err != nil {
		// Neither a transport error nor remote output is safe to return here:
		// Dokku tracing or remote command errors can contain credentials.
		return "", fmt.Errorf("postgres:%s failed; inspect the server without exposing credentials", verb)
	}
	return output, nil
}

func readPostgresBackup(ctx context.Context, run postgresBackupRunner, service string) (postgresBackupState, error) {
	var state postgresBackupState
	output, err := runBackupOperation(ctx, run, false, "info", service)
	if err != nil {
		// A filtered service list and an exit status do not prove deletion:
		// permission and transport failures must never clear Terraform state.
		return state, err
	}
	values := make(map[string]string)
	for _, line := range strings.Split(output, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		key = strings.ReplaceAll(key, "-", " ")
		if !strings.HasPrefix(key, "backup ") {
			continue
		}
		if _, exists := values[key]; exists {
			return state, errors.New("ambiguous duplicate fields in postgres backup report")
		}
		values[key] = strings.TrimSpace(value)
	}
	for _, key := range []string{"backup schedule", "backup schedules", "backup bucket", "backup authenticated", "backup auth fingerprint", "backup access key id fingerprint", "backup secret access key fingerprint", "backup default region", "backup signature version", "backup endpoint url", "backup use iam"} {
		if _, ok := values[key]; !ok {
			return state, fmt.Errorf("postgres plugin does not report required field %q; upgrade to a version with backup schedule and fingerprint reporting", key)
		}
	}
	state.ServiceExists = true
	state.Schedule = values["backup schedule"]
	state.Bucket = values["backup bucket"]
	state.Region = values["backup default region"]
	state.SignatureVersion = values["backup signature version"]
	state.Endpoint = values["backup endpoint url"]
	state.AuthFingerprint = values["backup auth fingerprint"]
	state.AccessKeyFingerprint = values["backup access key id fingerprint"]
	state.SecretKeyFingerprint = values["backup secret access key fingerprint"]
	state.AuthPartial = (state.AccessKeyFingerprint == "") != (state.SecretKeyFingerprint == "")
	state.Authenticated, err = strconv.ParseBool(values["backup authenticated"])
	if err != nil {
		return postgresBackupState{}, errors.New("invalid backup authentication state in postgres report")
	}
	state.UseIAM, err = strconv.ParseBool(values["backup use iam"])
	if err != nil {
		return postgresBackupState{}, errors.New("invalid backup IAM state in postgres report")
	}
	if state.UseIAM {
		return postgresBackupState{}, errors.New("the existing backup uses IAM; refusing to manage its authentication")
	}
	schedules := values["backup schedules"]
	state.Scheduled = state.Schedule != "" || state.Bucket != ""
	if schedules != "" {
		entries := strings.Split(schedules, ";")
		if len(entries) != 1 {
			return postgresBackupState{}, errors.New("multiple backup schedules exist; refusing to modify shared backup settings")
		}
		parts := strings.Split(entries[0], "|")
		if len(parts) != 4 || parts[0] != "default" {
			return postgresBackupState{}, errors.New("a named or invalid backup schedule exists; refusing to modify shared backup settings")
		}
		if parts[1] != state.Schedule || parts[2] != state.Bucket || parts[3] != "false" {
			return postgresBackupState{}, errors.New("inconsistent backup schedule report; refusing to modify shared backup settings")
		}
	} else if state.Scheduled {
		return postgresBackupState{}, errors.New("inconsistent backup schedule report; refusing to modify shared backup settings")
	}
	if (state.Schedule == "") != (state.Bucket == "") {
		return postgresBackupState{}, errors.New("incomplete backup schedule report")
	}
	if state.Authenticated && (state.AuthFingerprint == "" || state.AccessKeyFingerprint == "" || state.SecretKeyFingerprint == "") {
		return postgresBackupState{}, errors.New("backup credentials exist but their fingerprint is unavailable")
	}
	if !state.Authenticated && state.AccessKeyFingerprint != "" && state.SecretKeyFingerprint != "" {
		return postgresBackupState{}, errors.New("inconsistent backup authentication report")
	}
	return state, nil
}

func postgresBackupValueFingerprint(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func postgresBackupFingerprint(access, secret string) string {
	digest := sha256.Sum256([]byte(access + "\n" + secret))
	return hex.EncodeToString(digest[:])
}

func authPostgresBackup(ctx context.Context, run postgresBackupRunner, service, access, secret, region, signature, endpoint string) error {
	if access == "" || secret == "" || strings.ContainsAny(access+secret, "\r\n") {
		return errors.New("backup access key and secret must be nonempty single-line values")
	}
	_, err := runBackupOperation(ctx, run, true, "backup-auth", service, access, secret, region, signature, endpoint)
	return err
}
func schedulePostgresBackup(ctx context.Context, run postgresBackupRunner, service, schedule, bucket string) error {
	_, err := runBackupOperation(ctx, run, false, "backup-schedule", service, schedule, bucket)
	return err
}
func unschedulePostgresBackup(ctx context.Context, run postgresBackupRunner, service string) error {
	_, err := runBackupOperation(ctx, run, false, "backup-unschedule", service)
	return err
}
func deauthPostgresBackup(ctx context.Context, run postgresBackupRunner, service string) error {
	_, err := runBackupOperation(ctx, run, false, "backup-deauth", service)
	return err
}
