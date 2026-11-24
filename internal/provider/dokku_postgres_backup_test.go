package provider

import (
	"context"
	"errors"
	"strings"
	"testing"
)

const backupInfoFixture = `=====> test-db postgres service information
Backup schedule: 0 3 * * *
Backup schedules: default|0 3 * * *|backup-test|false
Backup bucket: backup-test
Backup authenticated: true
Backup auth fingerprint: 123456
Backup access key id fingerprint: access123
Backup secret access key fingerprint: secret123
Backup default region: us-east-1
Backup signature version: s3v4
Backup endpoint url: https://example.test:9000
Backup use iam: false
`

func TestReadPostgresBackupRejectsAmbiguousReports(t *testing.T) {
	cases := []struct {
		name, report string
		ok           bool
	}{
		{"valid", backupInfoFixture, true},
		{"missing field", strings.Replace(backupInfoFixture, "Backup schedules: default|0 3 * * *|backup-test|false\n", "", 1), false},
		{"duplicate field", backupInfoFixture + "Backup bucket: other\n", false},
		{"multiple schedules", strings.Replace(backupInfoFixture, "default|0 3 * * *|backup-test|false", "default|0 3 * * *|backup-test|false;other|0 4 * * *|other|false", 1), false},
		{"named schedule", strings.Replace(backupInfoFixture, "default|0 3", "named|0 3", 1), false},
		{"inconsistent bucket", strings.Replace(backupInfoFixture, "default|0 3 * * *|backup-test|false", "default|0 3 * * *|other|false", 1), false},
		{"iam", strings.Replace(backupInfoFixture, "Backup use iam: false", "Backup use iam: true", 1), false},
		{"missing fingerprint", strings.Replace(backupInfoFixture, "Backup auth fingerprint: 123456", "Backup auth fingerprint:", 1), false},
		{"invalid authenticated", strings.Replace(backupInfoFixture, "Backup authenticated: true", "Backup authenticated: maybe", 1), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runner := func(_ context.Context, cmd string, sensitive bool) (string, error) {
				if !strings.HasPrefix(cmd, "postgres:info ") || sensitive {
					t.Fatalf("unexpected command %q", cmd)
				}
				return tc.report, nil
			}
			state, err := readPostgresBackup(context.Background(), runner, "test-db")
			if (err == nil) != tc.ok {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.ok && (!state.ServiceExists || !state.Scheduled || state.Bucket != "backup-test" || state.Endpoint != "https://example.test:9000") {
				t.Fatalf("incorrect state: %#v", state)
			}
		})
	}
}

func TestReadPostgresBackupPreservesStateOnInfoError(t *testing.T) {
	calls := 0
	runner := func(_ context.Context, cmd string, _ bool) (string, error) {
		calls++
		if !strings.HasPrefix(cmd, "postgres:info ") {
			t.Fatalf("unsafe fallback after info failure: %s", cmd)
		}
		return "", errors.New("service unavailable or permission denied")
	}
	_, err := readPostgresBackup(context.Background(), runner, "test-db")
	if err == nil || calls != 1 {
		t.Fatalf("expected a single failed read, got %d calls and %v", calls, err)
	}
}

func TestBackupCommandsQuoteArgumentsAndRedactFailures(t *testing.T) {
	service, access, secret := "db name", "key with space", "secret'$(touch /tmp/no)"
	runner := func(_ context.Context, command string, sensitive bool) (string, error) {
		if !sensitive {
			t.Fatal("authentication command not marked sensitive")
		}
		words, err := splitDokkuCommand(command)
		if err != nil {
			t.Fatal(err)
		}
		if len(words) > 1 && words[1] == "--" {
			words = append(words[:1], words[2:]...)
		}
		if len(words) != 7 || words[1] != service || words[2] != access || words[3] != secret {
			t.Fatalf("arguments not preserved: %#v", words)
		}
		return secret, errors.New("transport error with " + secret)
	}
	err := authPostgresBackup(context.Background(), runner, service, access, secret, "us-east-1", "s3v4", "https://example.test")
	if err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("credential leaked in error: %v", err)
	}
	for _, invalid := range []string{"", "bad\nkey", "bad\rkey"} {
		if err := authPostgresBackup(context.Background(), runner, service, invalid, secret, "us-east-1", "s3v4", ""); err == nil {
			t.Fatalf("accepted invalid access key %q", invalid)
		}
	}
	if _, err := postgresBackupCommand("info", "bad\x00name"); err == nil {
		t.Fatal("accepted NUL in argument")
	}
}

func TestPostgresBackupSchemaPreservesExistingResources(t *testing.T) {
	provider := Provider()
	if err := provider.InternalValidate(); err != nil {
		t.Fatalf("provider schema: %v", err)
	}
	for _, name := range []string{"dokku_app", "dokku_postgres_service", "dokku_postgres_service_link", "dokku_redis_service", "dokku_redis_service_link", "dokku_mysql_service", "dokku_mysql_service_link", "dokku_clickhouse_service", "dokku_clickhouse_service_link"} {
		if provider.ResourcesMap[name] == nil {
			t.Errorf("existing resource %s missing", name)
		}
	}
	resource := provider.ResourcesMap["dokku_postgres_backup"]
	if resource == nil {
		t.Fatal("backup resource missing")
	}
	for _, name := range []string{"access_key_id_wo", "secret_access_key_wo"} {
		field := resource.Schema[name]
		if field == nil || !field.Required || !field.Sensitive || !field.WriteOnly {
			t.Errorf("%s must be required, sensitive and write-only", name)
		}
	}
	if !resource.Schema["service_name"].ForceNew {
		t.Error("service_name must identify the resource")
	}
	for _, name := range []string{"schedule", "bucket", "credentials_version"} {
		if resource.Schema[name].ForceNew {
			t.Errorf("%s must not require replacement", name)
		}
	}
}
