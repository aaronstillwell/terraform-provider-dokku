package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/melbahja/goph"
	"golang.org/x/crypto/ssh"
)

func TestDokkuLogsCommandQuotingAndControlChars(t *testing.T) {
	dsn := "datadog_logs://?api_key=abc&tags[]=a'b&site=datadoghq.com"
	cmd, err := dokkuLogsCommand("logs:set", "my-app", "vector-sink", dsn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	words, err := splitDokkuCommand(cmd)
	if err != nil {
		t.Fatalf("splitDokkuCommand failed: %v", err)
	}
	if len(words) != 4 || words[0] != "logs:set" || words[1] != "my-app" || words[2] != "vector-sink" || words[3] != dsn {
		t.Fatalf("arguments not round-tripped cleanly: %#v", words)
	}

	for _, bad := range []string{"bad\x00val", "bad\nval", "bad\rval"} {
		if _, err := dokkuLogsCommand("logs:set", "--global", "max-size", bad); err == nil {
			t.Fatalf("expected control char rejection for %q", bad)
		}
	}
	if _, err := dokkuLogsCommand("logs:set\n", "--global"); err == nil {
		t.Fatal("expected verb control char rejection")
	}
}

func TestLogsValidation(t *testing.T) {
	validMaxSizes := []string{"", "10m", "512k", "2g", "100M", "unlimited"}
	for _, v := range validMaxSizes {
		if _, errs := validateLogsMaxSize(v, "max_size"); len(errs) != 0 {
			t.Errorf("expected valid max_size %q, got %v", v, errs)
		}
	}
	for _, v := range []string{"0m", "10mb", "-5m", "10 m", "foo", "10m\n"} {
		if _, errs := validateLogsMaxSize(v, "max_size"); len(errs) == 0 {
			t.Errorf("expected invalid max_size %q", v)
		}
	}

	for _, v := range []string{"", "service", "com.dokku.app-name", "app_label_1"} {
		if _, errs := validateLogsAppLabelAlias(v, "app_label_alias"); len(errs) != 0 {
			t.Errorf("expected valid app_label_alias %q, got %v", v, errs)
		}
	}
	for _, v := range []string{"-bad", ".bad", "bad space", "bad\n", "bad/slash"} {
		if _, errs := validateLogsAppLabelAlias(v, "app_label_alias"); len(errs) == 0 {
			t.Errorf("expected invalid app_label_alias %q", v)
		}
	}

	for _, v := range []string{"", "timberio/vector:0.61.0-debian", "ghcr.io/org/vector:latest"} {
		if _, errs := validateLogsVectorImage(v, "vector_image"); len(errs) != 0 {
			t.Errorf("expected valid vector_image %q, got %v", v, errs)
		}
	}
	for _, v := range []string{"-bad", "bad image", "bad\timage", "bad\nimage"} {
		if _, errs := validateLogsVectorImage(v, "vector_image"); len(errs) == 0 {
			t.Errorf("expected invalid vector_image %q", v)
		}
	}

	for _, v := range []string{"internal-net", "app_net.1"} {
		if _, errs := validateLogsVectorNetworkItem(v, "vector_networks"); len(errs) != 0 {
			t.Errorf("expected valid network %q, got %v", v, errs)
		}
	}
	for _, v := range []string{"", "bridge", "a,b", "bad net", "bad\tnet", "bad\nnet", "-net"} {
		if _, errs := validateLogsVectorNetworkItem(v, "vector_networks"); len(errs) == 0 {
			t.Errorf("expected invalid network %q", v)
		}
	}

	for _, v := range []string{"", "console://", "datadog_logs://?api_key=secret&site=datadoghq.com", "loki://?auth[token]=abc"} {
		if _, errs := validateLogsVectorSink(v, "vector_sink"); len(errs) != 0 {
			t.Errorf("expected valid vector_sink %q, got %v", v, errs)
		}
	}
	for _, v := range []string{"not-a-dsn", "://missing-scheme", "console://?sinks=foo", "http://?a=1 b=2", "-console://", "console://\n"} {
		if _, errs := validateLogsVectorSink(v, "vector_sink"); len(errs) == 0 {
			t.Errorf("expected invalid vector_sink %q", v)
		}
	}

	for _, v := range []string{"my-app", "app_1.test"} {
		if _, errs := validateLogsAppName(v, "app"); len(errs) != 0 {
			t.Errorf("expected valid app %q, got %v", v, errs)
		}
	}
	for _, v := range []string{"", "-app", "bad app", "bad\napp"} {
		if _, errs := validateLogsAppName(v, "app"); len(errs) == 0 {
			t.Errorf("expected invalid app %q", v)
		}
	}
}

func TestParseLogsReportJSONCanonicalAndLegacyFallback(t *testing.T) {
	canonicalJSON := `{
		"app-label-alias": "service_name",
		"computed-app-label-alias": "service_name",
		"computed-max-size": "50m",
		"computed-vector-cron-sink": "loki://?auth[token]=cron_secret",
		"computed-vector-image": "timberio/vector:0.61.0-debian",
		"computed-vector-networks": "net-b,net-a",
		"computed-vector-sink": "datadog_logs://?api_key=app_secret",
		"global-app-label-alias": "com.dokku.app-name",
		"global-max-size": "25m",
		"global-vector-cron-sink": "loki://?auth[token]=global_cron_secret",
		"global-vector-image": "timberio/vector:0.61.0-debian",
		"global-vector-networks": "net-b, net-a, net-b",
		"global-vector-sink": "datadog_logs://?api_key=global_secret",
		"max-size": "50m",
		"vector-cron-sink": "loki://?auth[token]=cron_secret",
		"vector-sink": "datadog_logs://?api_key=app_secret"
	}`

	r, err := parseLogsReportJSON(canonicalJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.MaxSize != "50m" || r.GlobalMaxSize != "25m" || r.ComputedMaxSize != "50m" {
		t.Fatalf("unexpected max-size fields: %#v", r)
	}
	if r.AppLabelAlias != "service_name" || r.GlobalAppLabelAlias != "com.dokku.app-name" || r.ComputedAppLabelAlias != "service_name" {
		t.Fatalf("unexpected app-label-alias fields: %#v", r)
	}
	if r.VectorSink != "datadog_logs://?api_key=app_secret" || r.GlobalVectorSink != "datadog_logs://?api_key=global_secret" {
		t.Fatalf("unexpected vector-sink fields: %#v", r)
	}
	if r.VectorCronSink != "loki://?auth[token]=cron_secret" || r.GlobalVectorCronSink != "loki://?auth[token]=global_cron_secret" {
		t.Fatalf("unexpected vector-cron-sink fields: %#v", r)
	}
	if r.GlobalVectorImage != "timberio/vector:0.61.0-debian" || r.ComputedVectorImage != "timberio/vector:0.61.0-debian" {
		t.Fatalf("unexpected vector-image fields: %#v", r)
	}
	if strings.Join(r.GlobalVectorNetworks, ",") != "net-a,net-b" || strings.Join(r.ComputedVectorNetworks, ",") != "net-a,net-b" {
		t.Fatalf("unexpected vector-networks fields: %#v", r)
	}

	legacyJSON := `{
		"logs-max-size": "20m",
		"logs-global-max-size": "10m",
		"logs-computed-max-size": "20m",
		"logs-app-label-alias": "app",
		"logs-global-app-label-alias": "global_app",
		"logs-computed-app-label-alias": "app",
		"logs-vector-sink": "console://",
		"logs-global-vector-sink": "http://?uri=https://logs.example",
		"logs-computed-vector-sink": "console://",
		"logs-vector-cron-sink": "console://?cron=1",
		"logs-global-vector-cron-sink": "console://?global=1",
		"logs-computed-vector-cron-sink": "console://?cron=1",
		"logs-vector-global-image": "timberio/vector:0.50.0-debian",
		"logs-computed-vector-image": "timberio/vector:0.50.0-debian",
		"logs-global-vector-networks": "zeta,alpha",
		"logs-computed-vector-networks": "zeta,alpha"
	}`
	lr, err := parseLogsReportJSON(legacyJSON)
	if err != nil {
		t.Fatalf("legacy parse failed: %v", err)
	}
	if lr.MaxSize != "20m" || lr.GlobalVectorImage != "timberio/vector:0.50.0-debian" || strings.Join(lr.GlobalVectorNetworks, ",") != "alpha,zeta" {
		t.Fatalf("unexpected legacy parsed fields: %#v", lr)
	}

	// Ensure invalid JSON errors never leak embedded secret DSNs.
	const secret = "LOGS_DSN_SECRET_999"
	_, err = parseLogsReportJSON(`{"global-vector-sink": "datadog_logs://?api_key=` + secret + `"`)
	if err == nil {
		t.Fatal("expected JSON parse error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("secret leaked in JSON parse error: %v", err)
	}
}

func TestReadLogsVectorRunningParsing(t *testing.T) {
	cases := []struct {
		name    string
		res     dokkuLogsResult
		err     error
		want    bool
		wantErr bool
	}{
		{
			name: "running",
			res:  dokkuLogsResult{stdout: "vector log line\n", status: 0},
			want: true,
		},
		{
			name: "stopped warning exit 0",
			res:  dokkuLogsResult{stderr: " !     Vector container is not running\n", status: 0},
			want: false,
		},
		{
			name: "missing container exit 1",
			res:  dokkuLogsResult{stderr: " !     Vector container does not exist\n", status: 1},
			err:  errors.New("logs SSH command exited with status 1"),
			want: false,
		},
		{
			name:    "unexpected remote exit 1",
			res:     dokkuLogsResult{stderr: "permission denied", status: 1},
			err:     errors.New("logs SSH command exited with status 1"),
			wantErr: true,
		},
		{
			name:    "transport error",
			res:     dokkuLogsResult{status: 0},
			err:     errors.New("connection reset"),
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runner := func(_ context.Context, cmd string, sensitive bool) (dokkuLogsResult, error) {
				if cmd != "logs:vector-logs --num 1" || !sensitive {
					t.Fatalf("unexpected command %q sensitive=%t", cmd, sensitive)
				}
				return tc.res, tc.err
			}
			got, err := readLogsVectorRunning(context.Background(), runner)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%t", err, tc.wantErr)
			}
			if !tc.wantErr && got != tc.want {
				t.Fatalf("got=%t want=%t", got, tc.want)
			}
		})
	}
}

func TestLogsSinkFingerprint(t *testing.T) {
	if got := logsSinkFingerprint(""); got != "" {
		t.Fatalf("expected empty fingerprint for empty DSN, got %q", got)
	}
	dsn := "datadog_logs://?api_key=secret"
	sum := sha256.Sum256([]byte(dsn))
	want := hex.EncodeToString(sum[:])
	if got := logsSinkFingerprint(dsn); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestLogsCommandsDoNotLeakSecretsInLogsOrErrors(t *testing.T) {
	const secretDSN = "datadog_logs://?api_key=SECRET_KEY_42XYZ&site=datadoghq.com"
	const cronDSN = "loki://?auth[token]=CRON_SECRET_77ABC"

	for _, cmd := range []string{
		"logs:report --global --format json",
		"logs:set --global vector-sink 'datadog_logs://?api_key=x'",
		"logs:vector-logs --num 1",
	} {
		if !sensitiveSSHOutput(cmd) {
			t.Errorf("sensitiveSSHOutput(%q) = false, want true", cmd)
		}
	}

	// Verify runLogsCommand over a real SSH channel never logs commands or output,
	// and never leaks remote output in returned errors.
	for _, status := range []uint32{0, 1} {
		t.Run(fmt.Sprintf("ssh_status_%d", status), func(t *testing.T) {
			logs := captureLogs(t)
			client := testSSHClient(t, &ssh.ServerConfig{NoClientAuth: true}, nil, func(newChannel ssh.NewChannel) {
				channel, requests, err := newChannel.Accept()
				if err != nil {
					t.Errorf("accept: %v", err)
					return
				}
				defer channel.Close()
				for req := range requests {
					if req.Type != "exec" {
						_ = req.Reply(false, nil)
						continue
					}
					_ = req.Reply(true, nil)
					_, _ = channel.Write([]byte(`{"global-vector-sink":"` + secretDSN + `","global-vector-cron-sink":"` + cronDSN + `"}`))
					_, _ = channel.Stderr().Write([]byte("echoing " + secretDSN + "\n"))
					_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{status}))
					return
				}
			})

			runner := newDokkuLogsRunner(client)
			if status == 0 {
				report, err := readGlobalLogsReport(context.Background(), runner)
				if err != nil {
					t.Fatalf("readGlobalLogsReport failed: %v", err)
				}
				if report.GlobalVectorSink != secretDSN || report.GlobalVectorCronSink != cronDSN {
					t.Fatalf("unexpected report: %#v", report)
				}
			} else {
				_, err := readGlobalLogsReport(context.Background(), runner)
				if err == nil {
					t.Fatal("expected error on non-zero exit")
				}
				if strings.Contains(err.Error(), secretDSN) || strings.Contains(err.Error(), cronDSN) {
					t.Fatalf("secret leaked in error: %v", err)
				}
			}
			if err := setGlobalLogProperty(context.Background(), runner, "vector-sink", secretDSN, true); status != 0 && err == nil {
				t.Fatal("expected setGlobalLogProperty error on status 1")
			} else if err != nil && strings.Contains(err.Error(), secretDSN) {
				t.Fatalf("secret leaked in set error: %v", err)
			}

			for _, s := range []string{secretDSN, cronDSN, "SECRET_KEY_42XYZ", "CRON_SECRET_77ABC", "logs:report", "logs:set"} {
				if strings.Contains(logs.String(), s) {
					t.Fatalf("sensitive data %q leaked in debug logs:\n%s", s, logs.String())
				}
			}
		})
	}
}

func TestProviderAcceptsDokku038Version(t *testing.T) {
	// Exercise providerConfigure against an in-process SSH server reporting Dokku 0.38.34.
	pemBytes := generateEd25519PEM(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	signer, err := ssh.ParsePrivateKey(pemBytes)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &ssh.ServerConfig{NoClientAuth: true}
	cfg.AddHostKey(signer)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				sc, chans, reqs, err := ssh.NewServerConn(conn, cfg)
				if err != nil {
					return
				}
				defer sc.Close()
				go ssh.DiscardRequests(reqs)
				for ch := range chans {
					channel, creqs, err := ch.Accept()
					if err != nil {
						continue
					}
					for req := range creqs {
						if req.Type == "exec" {
							_ = req.Reply(true, nil)
							_, _ = channel.Write([]byte("dokku version 0.38.34\n"))
							_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
							_ = channel.Close()
							break
						}
						_ = req.Reply(false, nil)
					}
				}
			}()
		}
	}()

	addr := listener.Addr().(*net.TCPAddr)
	d := schema.TestResourceDataRaw(t, Provider().Schema, map[string]interface{}{
		"ssh_host":                 addr.IP.String(),
		"ssh_user":                 "dokku",
		"ssh_port":                 addr.Port,
		"ssh_cert":                 string(pemBytes),
		"skip_known_hosts_check":   true,
		"fail_on_untested_version": true,
	})
	rawClient, diags := providerConfigure(context.Background(), d)
	if diags.HasError() {
		t.Fatalf("expected Dokku 0.38.34 to be accepted, got diags: %#v", diags)
	}
	client, ok := rawClient.(*goph.Client)
	if !ok || client == nil {
		t.Fatal("expected non-nil *goph.Client")
	}
	t.Cleanup(func() { _ = client.Close() })
}
