package provider

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
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

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"golang.org/x/crypto/ssh"
)

type fakeLogsHost struct {
	mu                     sync.Mutex
	apps                   map[string]bool
	globalMaxSize          string
	globalAppLabelAlias    string
	globalVectorImage      string
	globalVectorNetworks   string
	globalVectorSink       string
	globalVectorCronSink   string
	vectorRunning          bool
	vectorContainerCreated bool
	appProps               map[string]map[string]string
	commands               []string
	undeployedApps         map[string]bool
	failReport             bool
	failRebuildTransport   bool
}

func newFakeLogsHost() *fakeLogsHost {
	return &fakeLogsHost{
		apps:           map[string]bool{"my-app": true},
		appProps:       map[string]map[string]string{},
		undeployedApps: map[string]bool{},
	}
}

func (f *fakeLogsHost) runner() dokkuLogsRunner {
	return func(_ context.Context, command string, _ bool) (dokkuLogsResult, error) {
		out, errOut, status := f.execute(command)
		res := dokkuLogsResult{
			stdout: out,
			stderr: errOut,
			status: int(status),
		}
		if status > 0 {
			return res, fmt.Errorf("logs SSH command exited with status %d", status)
		}
		if errOut == "TRANSPORT_ERROR" {
			return dokkuLogsResult{}, errors.New("logs SSH command failed")
		}
		return res, nil
	}
}

func (f *fakeLogsHost) execute(command string) (stdout string, stderr string, status uint32) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.commands = append(f.commands, command)

	words, err := splitDokkuCommand(command)
	if err != nil || len(words) == 0 {
		return "", "invalid command", 1
	}

	switch words[0] {
	case "version":
		return "dokku version 0.38.34\n", "", 0
	case "apps:exists":
		if len(words) != 2 {
			return "", "bad arity", 1
		}
		if f.apps[words[1]] {
			return "", "", 0
		}
		return "", "App does not exist", 1
	case "ps:rebuild":
		if len(words) != 2 {
			return "", "bad arity", 1
		}
		if f.failRebuildTransport {
			return "", "TRANSPORT_ERROR", 0
		}
		if f.undeployedApps[words[1]] {
			return "", "App has not been deployed", 1
		}
		return "Rebuilt " + words[1] + "\n", "", 0
	case "logs:vector-start":
		f.vectorRunning = true
		f.vectorContainerCreated = true
		return "Vector started\n", "", 0
	case "logs:vector-stop":
		f.vectorRunning = false
		return "Vector stopped\n", "", 0
	case "logs:vector-logs":
		if !f.vectorContainerCreated && !f.vectorRunning {
			return "", " !     Vector container does not exist\n", 1
		}
		if !f.vectorRunning {
			return "", " !     Vector container is not running\n", 0
		}
		return "vector 0.61.0 running\n", "", 0
	case "logs:set":
		if len(words) < 3 {
			return "", "bad logs:set arity", 1
		}
		if words[1] == "--global" {
			prop := words[2]
			val := ""
			if len(words) >= 4 {
				val = words[3]
			}
			switch prop {
			case "max-size":
				f.globalMaxSize = val
			case "app-label-alias":
				f.globalAppLabelAlias = val
			case "vector-image":
				f.globalVectorImage = val
			case "vector-networks":
				f.globalVectorNetworks = val
			case "vector-sink":
				f.globalVectorSink = val
			case "vector-cron-sink":
				f.globalVectorCronSink = val
			default:
				return "", "unknown global property", 1
			}
			return fmt.Sprintf("Setting %s to %s\n", prop, val), "", 0
		}
		app := words[1]
		prop := words[2]
		val := ""
		if len(words) >= 4 {
			val = words[3]
		}
		if f.appProps[app] == nil {
			f.appProps[app] = map[string]string{}
		}
		f.appProps[app][prop] = val
		return fmt.Sprintf("Setting %s to %s\n", prop, val), "", 0
	case "logs:report":
		if f.failReport {
			return "", "report failed", 1
		}
		isGlobal := len(words) >= 2 && words[1] == "--global"
		app := ""
		if !isGlobal && len(words) >= 2 && !strings.HasPrefix(words[1], "--") {
			app = words[1]
		}
		computedMax := f.globalMaxSize
		if computedMax == "" {
			computedMax = "10m"
		}
		computedAlias := f.globalAppLabelAlias
		if computedAlias == "" {
			computedAlias = "com.dokku.app-name"
		}
		computedImage := f.globalVectorImage
		if computedImage == "" {
			computedImage = "timberio/vector:0.61.0-debian"
		}
		payload := map[string]string{
			"global-max-size":          f.globalMaxSize,
			"global-app-label-alias":   f.globalAppLabelAlias,
			"global-vector-image":      f.globalVectorImage,
			"global-vector-networks":   f.globalVectorNetworks,
			"global-vector-sink":       f.globalVectorSink,
			"global-vector-cron-sink":  f.globalVectorCronSink,
			"computed-vector-image":    computedImage,
			"computed-vector-networks": f.globalVectorNetworks,
		}
		if isGlobal {
			payload["computed-max-size"] = computedMax
			payload["computed-app-label-alias"] = computedAlias
			payload["computed-vector-sink"] = f.globalVectorSink
			payload["computed-vector-cron-sink"] = f.globalVectorCronSink
		} else {
			props := f.appProps[app]
			appMax := props["max-size"]
			appAlias := props["app-label-alias"]
			appSink := props["vector-sink"]
			appCronSink := props["vector-cron-sink"]
			if appMax != "" {
				computedMax = appMax
			}
			if appAlias != "" {
				computedAlias = appAlias
			}
			payload["max-size"] = appMax
			payload["app-label-alias"] = appAlias
			payload["vector-sink"] = appSink
			payload["vector-cron-sink"] = appCronSink
			payload["computed-max-size"] = computedMax
			payload["computed-app-label-alias"] = computedAlias
			payload["computed-vector-sink"] = appSink
			payload["computed-vector-cron-sink"] = appCronSink
		}
		b, _ := json.Marshal(payload)
		return string(b) + "\n", "", 0
	default:
		return "", "unexpected command", 1
	}
}

func TestLogsResourcesSchema(t *testing.T) {
	p := Provider()
	if err := p.InternalValidate(); err != nil {
		t.Fatalf("InternalValidate: %v", err)
	}

	globalRes := p.ResourcesMap["dokku_global_logs"]
	appRes := p.ResourcesMap["dokku_app_logs"]
	if globalRes == nil || appRes == nil {
		t.Fatal("logs resources missing from Provider().ResourcesMap")
	}

	for _, res := range []*schema.Resource{globalRes, appRes} {
		for _, prefix := range []string{"vector_sink", "vector_cron_sink"} {
			plain := res.Schema[prefix]
			wo := res.Schema[prefix+"_wo"]
			ver := res.Schema[prefix+"_version"]
			fp := res.Schema[prefix+"_fingerprint"]
			if plain == nil || wo == nil || ver == nil || fp == nil {
				t.Fatalf("missing %s attribute family", prefix)
			}
			if !plain.Sensitive || !wo.Sensitive || !wo.WriteOnly || !fp.Sensitive || !fp.Computed {
				t.Errorf("invalid sensitivity/write-only/computed flags on %s", prefix)
			}
			if ver.Default != nil {
				t.Errorf("%s_version must NOT have a Default (causes ConflictsWith errors)", prefix)
			}
		}
	}

	if !appRes.Schema["app"].Required || !appRes.Schema["app"].ForceNew {
		t.Error("dokku_app_logs.app must be Required and ForceNew")
	}
	if appRes.Schema["rebuild_on_max_size_change"].Default != true {
		t.Error("dokku_app_logs.rebuild_on_max_size_change must default to true")
	}
	if globalRes.Schema["vector_networks"].Type != schema.TypeSet {
		t.Error("dokku_global_logs.vector_networks must be TypeSet")
	}
}

func TestGlobalLogsLifecycleWithMockedRunner(t *testing.T) {
	host := newFakeLogsHost()
	run := host.runner()
	res := resourceGlobalLogs()

	// Create with plaintext sinks, networks (unsorted in input), and image.
	d := schema.TestResourceDataRaw(t, res.Schema, map[string]interface{}{
		"max_size":         "50m",
		"app_label_alias":  "service",
		"vector_image":     "timberio/vector:0.61.0-debian",
		"vector_networks":  []interface{}{"net-b", "net-a"},
		"vector_sink":      "console://",
		"vector_cron_sink": "console://?cron=1",
	})
	if diags := resourceGlobalLogsCreateWithRunner(context.Background(), d, run); diags.HasError() {
		t.Fatalf("create failed: %#v", diags)
	}
	if d.Id() != "global" {
		t.Fatalf("expected id 'global', got %q", d.Id())
	}
	if host.globalMaxSize != "50m" || host.globalVectorNetworks != "net-a,net-b" || host.vectorRunning {
		t.Fatalf("unexpected host state after create: %+v", host)
	}
	if d.Get("vector_sink_fingerprint").(string) != logsSinkFingerprint("console://") {
		t.Fatalf("unexpected fingerprint: %q", d.Get("vector_sink_fingerprint"))
	}

	// Read detects remote drift and preserves state on failure.
	host.globalMaxSize = "100m"
	if diags := resourceGlobalLogsReadWithRunner(context.Background(), d, run); diags.HasError() {
		t.Fatalf("read failed: %#v", diags)
	}
	if d.Get("max_size").(string) != "100m" {
		t.Fatalf("expected drifted max_size 100m, got %q", d.Get("max_size"))
	}

	host.failReport = true
	if diags := resourceGlobalLogsReadWithRunner(context.Background(), d, run); !diags.HasError() {
		t.Fatal("expected error when logs:report fails")
	}
	if d.Id() != "global" {
		t.Fatal("read error must not clear ID")
	}
	host.failReport = false

	// Import rejects non-global ID and accepts "global".
	badImport := res.Data(nil)
	badImport.SetId("not-global")
	if _, err := resourceGlobalLogsImportWithRunner(context.Background(), badImport, run); err == nil {
		t.Fatal("expected import error for non-global ID")
	}
	goodImport := res.Data(nil)
	goodImport.SetId("global")
	imported, err := resourceGlobalLogsImportWithRunner(context.Background(), goodImport, run)
	if err != nil || len(imported) != 1 || imported[0].Get("max_size").(string) != "100m" {
		t.Fatalf("import failed: err=%v imported=%#v", err, imported)
	}
	if imported[0].Get("owned_vector_running").(bool) {
		t.Fatal("import must set owned_vector_running=false")
	}

	// Delete with owned_vector_running=true and vector_running=true stops Vector and unsets all 6 properties.
	host.vectorRunning = true
	host.vectorContainerCreated = true
	_ = d.Set("owned_vector_running", true)
	_ = d.Set("vector_running", true)
	if diags := resourceGlobalLogsDeleteWithRunner(context.Background(), d, run); diags.HasError() {
		t.Fatalf("delete failed: %#v", diags)
	}
	if d.Id() != "" {
		t.Fatal("expected empty ID after delete")
	}
	if host.vectorRunning || host.globalMaxSize != "" || host.globalAppLabelAlias != "" ||
		host.globalVectorImage != "" || host.globalVectorNetworks != "" ||
		host.globalVectorSink != "" || host.globalVectorCronSink != "" {
		t.Fatalf("delete did not clear all global state: %+v", host)
	}
}

func TestReconcileGlobalVectorRunning(t *testing.T) {
	cases := []struct {
		name            string
		owned           bool
		desiredRunning  bool
		currentRunning  bool
		imageChanged    bool
		networksChanged bool
		wantCmds        []string
	}{
		{"owned start when stopped", true, true, false, false, false, []string{"logs:vector-start"}},
		{"owned start when stopped with image change", true, true, false, true, false, []string{"logs:vector-start"}},
		{"owned noop when already running", true, true, true, false, false, nil},
		{"owned restart when running and image changed", true, true, true, true, false, []string{"logs:vector-stop", "logs:vector-start"}},
		{"owned restart when running and networks changed", true, true, true, false, true, []string{"logs:vector-stop", "logs:vector-start"}},
		{"owned stop when running", true, false, true, false, false, []string{"logs:vector-stop"}},
		{"owned noop when stopped", true, false, false, true, true, nil},
		{"unowned noop when running without image/net change", false, false, true, false, false, nil},
		{"unowned restart when running and networks changed", false, false, true, false, true, []string{"logs:vector-stop", "logs:vector-start"}},
		{"unowned noop when stopped even if image changed", false, false, false, true, false, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			runner := func(_ context.Context, cmd string, _ bool) (dokkuLogsResult, error) {
				got = append(got, cmd)
				return dokkuLogsResult{}, nil
			}
			if err := reconcileGlobalVectorRunning(context.Background(), runner, tc.owned, tc.desiredRunning, tc.currentRunning, tc.imageChanged, tc.networksChanged); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if strings.Join(got, ",") != strings.Join(tc.wantCmds, ",") {
				t.Fatalf("commands = %v, want %v", got, tc.wantCmds)
			}
		})
	}
}

func TestAppLogsLifecycleWithMockedRunner(t *testing.T) {
	host := newFakeLogsHost()
	run := host.runner()
	res := resourceAppLogs()

	// Missing app fails on Create and Import.
	missingData := schema.TestResourceDataRaw(t, res.Schema, map[string]interface{}{
		"app":      "no-such-app",
		"max_size": "25m",
	})
	if diags := resourceAppLogsCreateWithRunner(context.Background(), missingData, run); !diags.HasError() {
		t.Fatal("expected create error for nonexistent app")
	}
	missingImport := res.Data(nil)
	missingImport.SetId("no-such-app")
	if _, err := resourceAppLogsImportWithRunner(context.Background(), missingImport, run); err == nil {
		t.Fatal("expected import error for nonexistent app")
	}

	// Undeployed app ignores non-zero ps:rebuild exit status.
	host.undeployedApps["my-app"] = true
	d := schema.TestResourceDataRaw(t, res.Schema, map[string]interface{}{
		"app":                        "my-app",
		"max_size":                   "50m",
		"app_label_alias":            "svc",
		"vector_sink":                "console://",
		"vector_cron_sink":           "console://?cron=1",
		"rebuild_on_max_size_change": true,
	})
	if diags := resourceAppLogsCreateWithRunner(context.Background(), d, run); diags.HasError() {
		t.Fatalf("create failed on undeployed app: %#v", diags)
	}
	if d.Id() != "my-app" || d.Get("computed_max_size").(string) != "50m" || d.Get("computed_app_label_alias").(string) != "svc" {
		t.Fatalf("unexpected app logs state: id=%q max=%q alias=%q", d.Id(), d.Get("computed_max_size"), d.Get("computed_app_label_alias"))
	}

	// Transport failure during ps:rebuild still fails.
	host.failRebuildTransport = true
	dNoRebuild := schema.TestResourceDataRaw(t, res.Schema, map[string]interface{}{
		"app":                        "my-app",
		"max_size":                   "60m",
		"rebuild_on_max_size_change": false,
	})
	// rebuild_on_max_size_change=false avoids ps:rebuild entirely.
	host.appProps["my-app"]["max-size"] = ""
	if diags := resourceAppLogsCreateWithRunner(context.Background(), dNoRebuild, run); diags.HasError() {
		t.Fatalf("unexpected error when rebuild_on_max_size_change=false: %#v", diags)
	}
	host.failRebuildTransport = false

	// Import sets rebuild_on_max_size_change=true.
	imp := res.Data(nil)
	imp.SetId("my-app")
	imported, err := resourceAppLogsImportWithRunner(context.Background(), imp, run)
	if err != nil || len(imported) != 1 {
		t.Fatalf("import failed: %v", err)
	}
	if !imported[0].Get("rebuild_on_max_size_change").(bool) {
		t.Fatal("expected rebuild_on_max_size_change=true after import")
	}

	// If app is deleted externally, Read clears ID without error.
	delete(host.apps, "my-app")
	if diags := resourceAppLogsReadWithRunner(context.Background(), d, run); diags.HasError() {
		t.Fatalf("read on deleted app failed: %#v", diags)
	}
	if d.Id() != "" {
		t.Fatal("expected Read to clear ID when app no longer exists")
	}

	// Re-add app and verify Delete unsets all 4 properties and runs ps:rebuild.
	host.apps["my-app"] = true
	host.undeployedApps["my-app"] = false
	d.SetId("my-app")
	_ = d.Set("max_size", "50m")
	_ = d.Set("rebuild_on_max_size_change", true)
	beforeCmds := len(host.commands)
	if diags := resourceAppLogsDeleteWithRunner(context.Background(), d, run); diags.HasError() {
		t.Fatalf("delete failed: %#v", diags)
	}
	if d.Id() != "" {
		t.Fatal("expected empty ID after delete")
	}
	sawRebuild := false
	for _, cmd := range host.commands[beforeCmds:] {
		if cmd == "ps:rebuild my-app" {
			sawRebuild = true
		}
	}
	if !sawRebuild {
		t.Fatalf("expected ps:rebuild on delete, commands=%v", host.commands[beforeCmds:])
	}
	for _, prop := range []string{"max-size", "app-label-alias", "vector-sink", "vector-cron-sink"} {
		if host.appProps["my-app"][prop] != "" {
			t.Fatalf("app property %s not unset: %q", prop, host.appProps["my-app"][prop])
		}
	}
}

func startFakeLogsSSH(t *testing.T, host *fakeLogsHost) int {
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
							stdout, stderr, status := host.execute(payload.Command)
							_, _ = io.WriteString(channel, stdout)
							_, _ = io.WriteString(channel.Stderr(), stderr)
							_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{status}))
							return
						}
					}(ch)
				}
			}()
		}
	}()
	t.Cleanup(func() { _ = listener.Close() })
	return listener.Addr().(*net.TCPAddr).Port
}

// TestLogsWriteOnlyCLI exercises dokku_global_logs and dokku_app_logs end-to-end
// over real Terraform/OpenTofu RPCs (WriteOnly attributes, GetRawConfigAt,
// owned_vector_running, drift warnings, import, and secret non-leakage).
func TestLogsWriteOnlyCLI(t *testing.T) {
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
			testLogsWriteOnlyCLI(t, cli)
		})
	}
}

func testLogsWriteOnlyCLI(t *testing.T, cli string) {
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

	host := newFakeLogsHost()
	// Pre-start an externally managed Vector container to verify owned_vector_running=false leaves it untouched.
	host.vectorRunning = true
	host.vectorContainerCreated = true
	port := startFakeLogsSSH(t, host)

	const globalSink1 = "datadog_logs://?api_key=GLOBAL_SINK_SECRET_111&site=datadoghq.com"
	const globalSink2 = "datadog_logs://?api_key=GLOBAL_SINK_SECRET_222&site=datadoghq.com"
	const globalCron1 = "loki://?auth[token]=GLOBAL_CRON_SECRET_333"
	const appSink1 = "datadog_logs://?api_key=APP_SINK_SECRET_444&site=datadoghq.com"
	const appSink2 = "datadog_logs://?api_key=APP_SINK_SECRET_555&site=datadoghq.com"
	allSecrets := []string{globalSink1, globalSink2, globalCron1, appSink1, appSink2, "GLOBAL_SINK_SECRET_111", "GLOBAL_SINK_SECRET_222", "GLOBAL_CRON_SECRET_333", "APP_SINK_SECRET_444", "APP_SINK_SECRET_555"}

	logPath := filepath.Join(dir, "debug.log")
	var env []string
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "TF_") || strings.HasPrefix(entry, "DOKKU_") || strings.HasPrefix(entry, "HOME=") {
			continue
		}
		env = append(env, entry)
	}
	env = append(env,
		"HOME="+home,
		"TF_CLI_CONFIG_FILE="+configFile,
		"TF_IN_AUTOMATION=1",
		"TF_LOG=DEBUG",
		"TF_LOG_PATH="+logPath,
		"TF_VAR_global_sink="+globalSink1,
		"TF_VAR_global_cron="+globalCron1,
		"TF_VAR_app_sink="+appSink1,
	)

	runCLI := func(wantSuccess bool, args ...string) []byte {
		t.Helper()
		cmd := exec.Command(cli, args...)
		cmd.Dir, cmd.Env = dir, env
		out, err := cmd.CombinedOutput()
		if (err == nil) != wantSuccess {
			t.Fatalf("%s %v: %v\n%s", cli, args, err, out)
		}
		return out
	}

	checkNoSecret := func(label string, data []byte) {
		t.Helper()
		for _, s := range allSecrets {
			if bytes.Contains(data, []byte(s)) {
				t.Fatalf("%s leaked secret %q", label, s)
			}
		}
	}

	writeTF := func(globalBody, appBody string) {
		t.Helper()
		cfg := fmt.Sprintf(`terraform {
  required_providers { dokku = { source = "aaronstillwell/dokku" } }
}
provider "dokku" {
  ssh_host = "127.0.0.1"
  ssh_user = "dokku"
  ssh_port = %d
  ssh_cert = %s
  skip_known_hosts_check = true
}
variable "global_sink" {
  type = string
  sensitive = true
  ephemeral = true
}
variable "global_cron" {
  type = string
  sensitive = true
  ephemeral = true
}
variable "app_sink" {
  type = string
  sensitive = true
  ephemeral = true
}
%s
%s
`, port, strconv.Quote(key), globalBody, appBody)
		if err := os.WriteFile(filepath.Join(dir, "main.tf"), []byte(cfg), 0600); err != nil {
			t.Fatal(err)
		}
	}

	// 1. Create global (without vector_running) and app logs (with _wo sinks).
	writeTF(`resource "dokku_global_logs" "default" {
  max_size                 = "50m"
  app_label_alias          = "service"
  vector_image             = "timberio/vector:0.61.0-debian"
  vector_networks          = ["net-b", "net-a"]
  vector_sink_wo           = var.global_sink
  vector_sink_version      = 1
  vector_cron_sink_wo      = var.global_cron
  vector_cron_sink_version = 1
}`, `resource "dokku_app_logs" "api" {
  app                 = "my-app"
  max_size            = "100m"
  vector_sink_wo      = var.app_sink
  vector_sink_version = 1
  vector_cron_sink    = "console://"
}`)

	runCLI(true, "plan", "-input=false", "-out=planfile", "-no-color")
	checkNoSecret("saved plan JSON", runCLI(true, "show", "-json", "planfile"))
	rawPlan, err := os.ReadFile(filepath.Join(dir, "planfile"))
	if err != nil {
		t.Fatal(err)
	}
	checkNoSecret("saved plan bytes", rawPlan)

	runCLI(true, "apply", "-input=false", "-auto-approve", "-no-color", "planfile")
	checkNoSecret("state JSON", runCLI(true, "show", "-json"))

	host.mu.Lock()
	if host.globalVectorSink != globalSink1 || host.globalVectorCronSink != globalCron1 || host.globalVectorNetworks != "net-a,net-b" {
		host.mu.Unlock()
		t.Fatalf("unexpected global host state: %+v", host)
	}
	if host.appProps["my-app"]["vector-sink"] != appSink1 || host.appProps["my-app"]["vector-cron-sink"] != "console://" {
		host.mu.Unlock()
		t.Fatalf("unexpected app host state: %+v", host.appProps["my-app"])
	}
	host.mu.Unlock()

	// 2. Changing ephemeral var without incrementing *_version produces "No changes".
	for i := range env {
		if strings.HasPrefix(env[i], "TF_VAR_global_sink=") {
			env[i] = "TF_VAR_global_sink=" + globalSink2
		}
		if strings.HasPrefix(env[i], "TF_VAR_app_sink=") {
			env[i] = "TF_VAR_app_sink=" + appSink2
		}
	}
	planOut := runCLI(true, "plan", "-input=false", "-no-color")
	if !bytes.Contains(planOut, []byte("No changes")) {
		t.Fatalf("expected No changes when only _wo value changed, got:\n%s", planOut)
	}

	// 3. Incrementing vector_sink_version rotates sinks.
	writeTF(`resource "dokku_global_logs" "default" {
  max_size                 = "50m"
  app_label_alias          = "service"
  vector_image             = "timberio/vector:0.61.0-debian"
  vector_networks          = ["net-b", "net-a"]
  vector_sink_wo           = var.global_sink
  vector_sink_version      = 2
  vector_cron_sink_wo      = var.global_cron
  vector_cron_sink_version = 1
}`, `resource "dokku_app_logs" "api" {
  app                 = "my-app"
  max_size            = "100m"
  vector_sink_wo      = var.app_sink
  vector_sink_version = 2
  vector_cron_sink    = "console://"
}`)
	runCLI(true, "apply", "-input=false", "-auto-approve", "-no-color")
	host.mu.Lock()
	if host.globalVectorSink != globalSink2 || host.appProps["my-app"]["vector-sink"] != appSink2 {
		host.mu.Unlock()
		t.Fatalf("rotation did not update remote sinks")
	}
	host.mu.Unlock()

	// 4. External drift on _wo sink emits warning and updates fingerprint without adopting plaintext sink.
	host.mu.Lock()
	host.appProps["my-app"]["vector-sink"] = "http://drifted.example"
	host.mu.Unlock()
	driftOut := runCLI(true, "plan", "-input=false", "-no-color")
	if !bytes.Contains(driftOut, []byte("vector_sink changed outside Terraform")) {
		t.Fatalf("expected drift warning, got:\n%s", driftOut)
	}
	if bytes.Contains(driftOut, []byte("http://drifted.example")) {
		t.Fatalf("plain vector_sink should not be adopted into state in _wo mode")
	}

	// 5. Destroy while vector_running was omitted (owned_vector_running=false) MUST NOT stop Vector.
	runCLI(true, "destroy", "-input=false", "-auto-approve", "-no-color")
	host.mu.Lock()
	if !host.vectorRunning {
		host.mu.Unlock()
		t.Fatal("destroy stopped externally started Vector when vector_running was omitted")
	}
	host.mu.Unlock()

	// 6. Manage vector_running=true explicitly, change vector_image -> verify stop+start, and verify destroy stops Vector.
	writeTF(`resource "dokku_global_logs" "default" {
  vector_image   = "timberio/vector:0.61.0-debian"
  vector_running = true
}`, ``)
	runCLI(true, "apply", "-input=false", "-auto-approve", "-no-color")

	host.mu.Lock()
	cmdStartIdx := len(host.commands)
	host.mu.Unlock()
	writeTF(`resource "dokku_global_logs" "default" {
  vector_image   = "timberio/vector:0.62.0-debian"
  vector_running = true
}`, ``)
	runCLI(true, "apply", "-input=false", "-auto-approve", "-no-color")
	host.mu.Lock()
	cmds := host.commands[cmdStartIdx:]
	host.mu.Unlock()
	stopIdx, startIdx := -1, -1
	for i, c := range cmds {
		if c == "logs:vector-stop" && stopIdx == -1 {
			stopIdx = i
		}
		if c == "logs:vector-start" && stopIdx != -1 {
			startIdx = i
		}
	}
	if stopIdx == -1 || startIdx == -1 || startIdx <= stopIdx {
		t.Fatalf("expected logs:vector-stop followed by logs:vector-start on image change, got %v", cmds)
	}

	// 7. Import plaintext sink, switch to _wo, then drop _wo + _version to 0 to unset sink.
	host.mu.Lock()
	host.appProps["my-app"]["vector-sink"] = "console://"
	host.mu.Unlock()
	writeTF(`resource "dokku_global_logs" "default" {
  vector_image   = "timberio/vector:0.62.0-debian"
  vector_running = true
}`, `resource "dokku_app_logs" "api" {
  app         = "my-app"
  vector_sink = "console://"
}`)
	runCLI(true, "import", "-input=false", "-no-color", "dokku_app_logs.api", "my-app")
	planAfterImport := runCLI(true, "plan", "-input=false", "-no-color")
	if !bytes.Contains(planAfterImport, []byte("No changes")) {
		t.Fatalf("expected No changes after import, got:\n%s", planAfterImport)
	}

	// Switch from plaintext to _wo.
	writeTF(`resource "dokku_global_logs" "default" {
  vector_image   = "timberio/vector:0.62.0-debian"
  vector_running = true
}`, `resource "dokku_app_logs" "api" {
  app                 = "my-app"
  vector_sink_wo      = var.app_sink
  vector_sink_version = 1
}`)
	runCLI(true, "apply", "-input=false", "-auto-approve", "-no-color")
	host.mu.Lock()
	if host.appProps["my-app"]["vector-sink"] != appSink2 {
		host.mu.Unlock()
		t.Fatalf("switch to _wo did not update sink")
	}
	host.mu.Unlock()

	// Remove _wo + _version -> unsets sink.
	writeTF(`resource "dokku_global_logs" "default" {
  vector_image   = "timberio/vector:0.62.0-debian"
  vector_running = true
}`, `resource "dokku_app_logs" "api" {
  app = "my-app"
}`)
	runCLI(true, "apply", "-input=false", "-auto-approve", "-no-color")
	host.mu.Lock()
	if host.appProps["my-app"]["vector-sink"] != "" {
		host.mu.Unlock()
		t.Fatalf("removing _wo did not unset remote sink, got %q", host.appProps["my-app"]["vector-sink"])
	}
	host.mu.Unlock()

	// Destroy with owned_vector_running=true stops Vector.
	runCLI(true, "destroy", "-input=false", "-auto-approve", "-no-color")
	host.mu.Lock()
	if host.vectorRunning {
		host.mu.Unlock()
		t.Fatal("destroy did not stop owned Vector container")
	}
	host.mu.Unlock()

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
