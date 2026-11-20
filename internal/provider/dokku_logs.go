package provider

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"al.essio.dev/pkg/shellescape"
	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/melbahja/goph"
	"golang.org/x/crypto/ssh"
)

var (
	logsAppNameRegex       = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)
	logsMaxSizeRegex       = regexp.MustCompile(`^(?:[1-9][0-9]*[kmgKMG]|unlimited)$`)
	logsAppLabelAliasRegex = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)
	logsSinkSchemeRegex    = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+_-]*$`)
)

// dokkuLogsResult holds the output and exit status of a Dokku logs command.
// Callers must never log stdout/stderr or embed them or sensitive arguments in errors.
type dokkuLogsResult struct {
	stdout string
	stderr string
	status int
}

// dokkuLogsRunner executes a Dokku CLI command without logging commands or output.
type dokkuLogsRunner func(ctx context.Context, command string, sensitive bool) (dokkuLogsResult, error)

func newDokkuLogsRunner(client *goph.Client) dokkuLogsRunner {
	return func(ctx context.Context, command string, _ bool) (dokkuLogsResult, error) {
		stdout, stderr, status, err := runLogsCommand(ctx, client, command)
		return dokkuLogsResult{
			stdout: stdout,
			stderr: stderr,
			status: status,
		}, err
	}
}

func dokkuLogsCommand(verb string, args ...string) (string, error) {
	if strings.ContainsAny(verb, "\x00\r\n") || strings.TrimSpace(verb) == "" {
		return "", errors.New("logs command verb is invalid")
	}
	parts := []string{verb}
	for _, arg := range args {
		if strings.ContainsAny(arg, "\x00\r\n") {
			return "", errors.New("logs command argument contains a disallowed control character")
		}
		parts = append(parts, shellescape.Quote(arg))
	}
	return strings.Join(parts, " "), nil
}

func validateLogsAppNameValue(v string) error {
	if strings.ContainsAny(v, "\x00\r\n") || !logsAppNameRegex.MatchString(v) {
		return errors.New("must match ^[A-Za-z0-9][A-Za-z0-9_.-]*$")
	}
	return nil
}

func validateLogsAppName(value interface{}, key string) ([]string, []error) {
	if err := validateLogsAppNameValue(value.(string)); err != nil {
		return nil, []error{fmt.Errorf("invalid %s: %w", key, err)}
	}
	return nil, nil
}

func validateLogsMaxSizeValue(v string) error {
	if v == "" {
		return nil
	}
	if strings.ContainsAny(v, "\x00\r\n") || !logsMaxSizeRegex.MatchString(v) {
		return errors.New("must be <int>[k|m|g] (positive integer) or \"unlimited\"")
	}
	return nil
}

func validateLogsMaxSize(value interface{}, key string) ([]string, []error) {
	if err := validateLogsMaxSizeValue(value.(string)); err != nil {
		return nil, []error{fmt.Errorf("invalid %s: %w", key, err)}
	}
	return nil, nil
}

func validateLogsAppLabelAliasValue(v string) error {
	if v == "" {
		return nil
	}
	if strings.ContainsAny(v, "\x00\r\n") || !logsAppLabelAliasRegex.MatchString(v) {
		return errors.New("must match ^[A-Za-z0-9][A-Za-z0-9_.-]*$")
	}
	return nil
}

func validateLogsAppLabelAlias(value interface{}, key string) ([]string, []error) {
	if err := validateLogsAppLabelAliasValue(value.(string)); err != nil {
		return nil, []error{fmt.Errorf("invalid %s: %w", key, err)}
	}
	return nil, nil
}

func validateLogsVectorImageValue(v string) error {
	if v == "" {
		return nil
	}
	if strings.ContainsAny(v, " \t\r\n\x00") || strings.HasPrefix(v, "-") {
		return errors.New("must not contain whitespace, control characters, or start with '-'")
	}
	return nil
}

func validateLogsVectorImage(value interface{}, key string) ([]string, []error) {
	if err := validateLogsVectorImageValue(value.(string)); err != nil {
		return nil, []error{fmt.Errorf("invalid %s: %w", key, err)}
	}
	return nil, nil
}

func validateLogsVectorNetworkItemValue(v string) error {
	if v == "" {
		return errors.New("network name must not be empty")
	}
	if v == "bridge" {
		return errors.New("\"bridge\" network is not allowed by Dokku")
	}
	if strings.Contains(v, ",") {
		return errors.New("network name must not contain commas")
	}
	if strings.ContainsAny(v, " \t\r\n\x00") || strings.HasPrefix(v, "-") {
		return errors.New("network name must not contain whitespace, control characters, or start with '-'")
	}
	return nil
}

func validateLogsVectorNetworkItem(value interface{}, key string) ([]string, []error) {
	if err := validateLogsVectorNetworkItemValue(value.(string)); err != nil {
		return nil, []error{fmt.Errorf("invalid %s: %w", key, err)}
	}
	return nil, nil
}

func validateLogsVectorSinkValue(v string) error {
	if v == "" {
		return nil
	}
	if strings.ContainsAny(v, " \t\r\n\x00") || strings.HasPrefix(v, "-") {
		return errors.New("must be a valid sink DSN without whitespace, control characters, or leading '-'")
	}
	scheme, rest, ok := strings.Cut(v, "://")
	if !ok || !logsSinkSchemeRegex.MatchString(scheme) {
		return errors.New("must use SINK_TYPE://?SINK_OPTIONS format")
	}
	// Go's net/url rejects '_' in schemes per RFC 3986, while Dokku Vector sinks
	// use schemes like datadog_logs:// and aws_s3://. Normalize '_' for parsing.
	normalized := strings.ReplaceAll(scheme, "_", "-") + "://" + rest
	u, err := url.Parse(normalized)
	if err != nil || u.Scheme == "" {
		return errors.New("must be a valid sink URI with a scheme")
	}
	if u.Query().Has("sinks") {
		return errors.New("the \"sinks\" query parameter is not permitted")
	}
	return nil
}

func validateLogsVectorSink(value interface{}, key string) ([]string, []error) {
	if err := validateLogsVectorSinkValue(value.(string)); err != nil {
		return nil, []error{fmt.Errorf("invalid %s: %w", key, err)}
	}
	return nil, nil
}

func logsSinkFingerprint(value string) string {
	if value == "" {
		return ""
	}
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

type dokkuLogsReport struct {
	MaxSize                string
	GlobalMaxSize          string
	ComputedMaxSize        string
	AppLabelAlias          string
	GlobalAppLabelAlias    string
	ComputedAppLabelAlias  string
	VectorSink             string
	GlobalVectorSink       string
	ComputedVectorSink     string
	VectorCronSink         string
	GlobalVectorCronSink   string
	ComputedVectorCronSink string
	GlobalVectorImage      string
	ComputedVectorImage    string
	GlobalVectorNetworks   []string
	ComputedVectorNetworks []string
}

func parseLogsNetworkList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}
	seen := make(map[string]struct{})
	var out []string
	for _, part := range strings.Split(raw, ",") {
		item := strings.TrimSpace(part)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	sort.Strings(out)
	return out
}

func joinLogsNetworkSet(s *schema.Set) string {
	if s == nil || s.Len() == 0 {
		return ""
	}
	seen := make(map[string]struct{}, s.Len())
	items := make([]string, 0, s.Len())
	for _, raw := range s.List() {
		v := raw.(string)
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		items = append(items, v)
	}
	sort.Strings(items)
	return strings.Join(items, ",")
}

func normalizeLogsNetworkSet(s *schema.Set) []string {
	if s == nil || s.Len() == 0 {
		return []string{}
	}
	return parseLogsNetworkList(joinLogsNetworkSet(s))
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func logsReportField(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		val, ok := m[k]
		if !ok || val == nil {
			continue
		}
		if s, ok := val.(string); ok {
			return s
		}
	}
	return ""
}

func parseLogsReportJSON(raw string) (dokkuLogsReport, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return dokkuLogsReport{}, errors.New("empty JSON output from logs:report")
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil || payload == nil {
		// Never include raw report output in errors because it may contain sink DSNs.
		return dokkuLogsReport{}, errors.New("failed to parse JSON output from logs:report")
	}

	return dokkuLogsReport{
		MaxSize:                logsReportField(payload, "max-size", "logs-max-size"),
		GlobalMaxSize:          logsReportField(payload, "global-max-size", "logs-global-max-size"),
		ComputedMaxSize:        logsReportField(payload, "computed-max-size", "logs-computed-max-size"),
		AppLabelAlias:          logsReportField(payload, "app-label-alias", "logs-app-label-alias"),
		GlobalAppLabelAlias:    logsReportField(payload, "global-app-label-alias", "logs-global-app-label-alias"),
		ComputedAppLabelAlias:  logsReportField(payload, "computed-app-label-alias", "logs-computed-app-label-alias"),
		VectorSink:             logsReportField(payload, "vector-sink", "logs-vector-sink"),
		GlobalVectorSink:       logsReportField(payload, "global-vector-sink", "logs-global-vector-sink"),
		ComputedVectorSink:     logsReportField(payload, "computed-vector-sink", "logs-computed-vector-sink"),
		VectorCronSink:         logsReportField(payload, "vector-cron-sink", "logs-vector-cron-sink"),
		GlobalVectorCronSink:   logsReportField(payload, "global-vector-cron-sink", "logs-global-vector-cron-sink"),
		ComputedVectorCronSink: logsReportField(payload, "computed-vector-cron-sink", "logs-computed-vector-cron-sink"),
		GlobalVectorImage:      logsReportField(payload, "global-vector-image", "logs-global-vector-image", "vector-global-image", "logs-vector-global-image"),
		ComputedVectorImage:    logsReportField(payload, "computed-vector-image", "logs-computed-vector-image"),
		GlobalVectorNetworks:   parseLogsNetworkList(logsReportField(payload, "global-vector-networks", "logs-global-vector-networks")),
		ComputedVectorNetworks: parseLogsNetworkList(logsReportField(payload, "computed-vector-networks", "logs-computed-vector-networks")),
	}, nil
}

func readGlobalLogsReport(ctx context.Context, run dokkuLogsRunner) (dokkuLogsReport, error) {
	cmd, err := dokkuLogsCommand("logs:report", "--global", "--format", "json")
	if err != nil {
		return dokkuLogsReport{}, err
	}
	res, err := run(ctx, cmd, true)
	if err != nil {
		if res.status > 0 {
			return dokkuLogsReport{}, fmt.Errorf("logs:report --global failed with status %d", res.status)
		}
		return dokkuLogsReport{}, errors.New("logs:report --global failed")
	}
	return parseLogsReportJSON(res.stdout)
}

func readAppLogsReport(ctx context.Context, run dokkuLogsRunner, app string) (dokkuLogsReport, error) {
	if err := validateLogsAppNameValue(app); err != nil {
		return dokkuLogsReport{}, fmt.Errorf("invalid app name: %w", err)
	}
	cmd, err := dokkuLogsCommand("logs:report", app, "--format", "json")
	if err != nil {
		return dokkuLogsReport{}, err
	}
	res, err := run(ctx, cmd, true)
	if err != nil {
		if res.status > 0 {
			return dokkuLogsReport{}, fmt.Errorf("logs:report failed with status %d", res.status)
		}
		return dokkuLogsReport{}, errors.New("logs:report failed")
	}
	return parseLogsReportJSON(res.stdout)
}

func checkDokkuAppExistsForLogs(ctx context.Context, run dokkuLogsRunner, app string) (bool, error) {
	if err := validateLogsAppNameValue(app); err != nil {
		return false, fmt.Errorf("invalid app name: %w", err)
	}
	cmd, err := dokkuLogsCommand("apps:exists", app)
	if err != nil {
		return false, err
	}
	res, err := run(ctx, cmd, false)
	if err != nil {
		if res.status > 0 {
			return false, nil
		}
		return false, errors.New("apps:exists failed")
	}
	return true, nil
}

func rebuildDokkuAppForLogs(ctx context.Context, run dokkuLogsRunner, app string) error {
	if err := validateLogsAppNameValue(app); err != nil {
		return fmt.Errorf("invalid app name: %w", err)
	}
	cmd, err := dokkuLogsCommand("ps:rebuild", app)
	if err != nil {
		return err
	}
	res, err := run(ctx, cmd, false)
	if err != nil {
		if res.status > 0 {
			// App may not have been deployed yet; max-size takes effect on next deploy.
			return nil
		}
		return errors.New("ps:rebuild failed")
	}
	return nil
}

func setGlobalLogProperty(ctx context.Context, run dokkuLogsRunner, prop, value string, sensitive bool) error {
	var (
		cmd string
		err error
	)
	if value == "" {
		cmd, err = dokkuLogsCommand("logs:set", "--global", prop)
	} else {
		cmd, err = dokkuLogsCommand("logs:set", "--global", prop, value)
	}
	if err != nil {
		return err
	}
	res, err := run(ctx, cmd, sensitive)
	if err != nil {
		if res.status > 0 {
			return fmt.Errorf("logs:set --global %s failed with status %d", prop, res.status)
		}
		return fmt.Errorf("logs:set --global %s failed", prop)
	}
	return nil
}

func setAppLogProperty(ctx context.Context, run dokkuLogsRunner, app, prop, value string, sensitive bool) error {
	if err := validateLogsAppNameValue(app); err != nil {
		return fmt.Errorf("invalid app name: %w", err)
	}
	var (
		cmd string
		err error
	)
	if value == "" {
		cmd, err = dokkuLogsCommand("logs:set", app, prop)
	} else {
		cmd, err = dokkuLogsCommand("logs:set", app, prop, value)
	}
	if err != nil {
		return err
	}
	res, err := run(ctx, cmd, sensitive)
	if err != nil {
		if res.status > 0 {
			return fmt.Errorf("logs:set %s failed with status %d", prop, res.status)
		}
		return fmt.Errorf("logs:set %s failed", prop)
	}
	return nil
}

func readLogsVectorRunning(ctx context.Context, run dokkuLogsRunner) (bool, error) {
	cmd, err := dokkuLogsCommand("logs:vector-logs", "--num", "1")
	if err != nil {
		return false, err
	}
	res, err := run(ctx, cmd, true)
	combined := res.stdout + "\n" + res.stderr
	if err != nil {
		if res.status > 0 && (strings.Contains(combined, "Vector container does not exist") || strings.Contains(combined, "Vector container is not running")) {
			return false, nil
		}
		if res.status > 0 {
			return false, fmt.Errorf("logs:vector-logs failed with status %d", res.status)
		}
		return false, errors.New("logs:vector-logs failed")
	}
	if strings.Contains(combined, "Vector container is not running") || strings.Contains(combined, "Vector container does not exist") {
		return false, nil
	}
	return true, nil
}

func startLogsVector(ctx context.Context, run dokkuLogsRunner) error {
	cmd, err := dokkuLogsCommand("logs:vector-start")
	if err != nil {
		return err
	}
	res, err := run(ctx, cmd, false)
	if err != nil {
		if res.status > 0 {
			return fmt.Errorf("logs:vector-start failed with status %d", res.status)
		}
		return errors.New("logs:vector-start failed")
	}
	return nil
}

func stopLogsVector(ctx context.Context, run dokkuLogsRunner) error {
	cmd, err := dokkuLogsCommand("logs:vector-stop")
	if err != nil {
		return err
	}
	res, err := run(ctx, cmd, false)
	if err != nil {
		if res.status > 0 {
			return fmt.Errorf("logs:vector-stop failed with status %d", res.status)
		}
		return errors.New("logs:vector-stop failed")
	}
	return nil
}

type logsSinkSpec struct {
	plainAttr   string
	woAttr      string
	versionAttr string
	fpAttr      string
	prop        string
}

var (
	vectorSinkSpec = logsSinkSpec{
		plainAttr:   "vector_sink",
		woAttr:      "vector_sink_wo",
		versionAttr: "vector_sink_version",
		fpAttr:      "vector_sink_fingerprint",
		prop:        "vector-sink",
	}
	vectorCronSinkSpec = logsSinkSpec{
		plainAttr:   "vector_cron_sink",
		woAttr:      "vector_cron_sink_wo",
		versionAttr: "vector_cron_sink_version",
		fpAttr:      "vector_cron_sink_fingerprint",
		prop:        "vector-cron-sink",
	}
)

func resolveLogsSinkConfig(d *schema.ResourceData, spec logsSinkSpec) (string, diag.Diagnostics) {
	version := d.Get(spec.versionAttr).(int)
	if version > 0 {
		woVal, diags := d.GetRawConfigAt(cty.GetAttrPath(spec.woAttr))
		if diags.HasError() {
			return "", diags
		}
		if !woVal.IsKnown() || woVal.IsNull() || woVal.Type() != cty.String || woVal.AsString() == "" {
			return "", diag.Errorf("%s must be a known, nonempty value when %s is set", spec.woAttr, spec.versionAttr)
		}
		dsn := woVal.AsString()
		if err := validateLogsVectorSinkValue(dsn); err != nil {
			return "", diag.Errorf("invalid %s: %v", spec.woAttr, err)
		}
		return dsn, nil
	}
	return d.Get(spec.plainAttr).(string), nil
}

func observeLogsSink(d *schema.ResourceData, spec logsSinkSpec, remoteValue string, warnOnDrift bool) diag.Diagnostics {
	version := d.Get(spec.versionAttr).(int)
	observedFP := logsSinkFingerprint(remoteValue)
	prevFP := d.Get(spec.fpAttr).(string)
	if version > 0 {
		// Never write a write-only sink DSN back into Terraform state.
		if err := d.Set(spec.fpAttr, observedFP); err != nil {
			return diag.FromErr(err)
		}
		if warnOnDrift && prevFP != observedFP {
			return diag.Diagnostics{{
				Severity: diag.Warning,
				Summary:  fmt.Sprintf("%s changed outside Terraform", spec.plainAttr),
				Detail:   fmt.Sprintf("Increment %s to explicitly reapply %s. The resource will not adopt the externally changed sink.", spec.versionAttr, spec.woAttr),
			}}
		}
		return nil
	}
	if err := d.Set(spec.plainAttr, remoteValue); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set(spec.fpAttr, observedFP); err != nil {
		return diag.FromErr(err)
	}
	return nil
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
