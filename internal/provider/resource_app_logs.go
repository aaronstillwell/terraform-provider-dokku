package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/melbahja/goph"
)

func resourceAppLogs() *schema.Resource {
	return &schema.Resource{
		Description:   "Manages per-application Dokku log configuration (`dokku logs:set <app>`).",
		CreateContext: resourceAppLogsCreate,
		ReadContext:   resourceAppLogsRead,
		UpdateContext: resourceAppLogsUpdate,
		DeleteContext: resourceAppLogsDelete,
		Importer:      &schema.ResourceImporter{StateContext: resourceAppLogsImport},
		Schema: map[string]*schema.Schema{
			"app": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validateLogsAppName,
				Description:  "Name of the existing Dokku application.",
			},
			"max_size": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validateLogsMaxSize,
				Description:  "Per-app container log max size (e.g. `50m`, `1g`, or `unlimited`). Omit to inherit the global Dokku setting.",
			},
			"app_label_alias": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validateLogsAppLabelAlias,
				Description:  "Per-app Vector label alias for the Dokku app name. Omit to inherit the global Dokku setting.",
			},
			"vector_sink": {
				Type:          schema.TypeString,
				Optional:      true,
				Sensitive:     true,
				ValidateFunc:  validateLogsVectorSink,
				ConflictsWith: []string{"vector_sink_wo", "vector_sink_version"},
				Description:   "Per-app Vector sink DSN (`SINK_TYPE://?SINK_OPTIONS`). Stored in Terraform state; prefer `vector_sink_wo` for DSNs containing secrets.",
			},
			"vector_sink_wo": {
				Type:          schema.TypeString,
				Optional:      true,
				Sensitive:     true,
				WriteOnly:     true,
				ValidateFunc:  validateLogsVectorSink,
				ConflictsWith: []string{"vector_sink"},
				RequiredWith:  []string{"vector_sink_version"},
				Description:   "Write-only per-app Vector sink DSN (`SINK_TYPE://?SINK_OPTIONS`); not saved in Terraform state (requires Terraform/OpenTofu >= 1.11).",
			},
			"vector_sink_version": {
				Type:          schema.TypeInt,
				Optional:      true,
				ValidateFunc:  validation.IntAtLeast(1),
				ConflictsWith: []string{"vector_sink"},
				RequiredWith:  []string{"vector_sink_wo"},
				Description:   "Increment to apply an updated `vector_sink_wo` DSN.",
			},
			"vector_sink_fingerprint": {
				Type:        schema.TypeString,
				Computed:    true,
				Sensitive:   true,
				Description: "SHA-256 hex digest of the remote app `vector-sink` DSN, or empty when unset.",
			},
			"vector_cron_sink": {
				Type:          schema.TypeString,
				Optional:      true,
				Sensitive:     true,
				ValidateFunc:  validateLogsVectorSink,
				ConflictsWith: []string{"vector_cron_sink_wo", "vector_cron_sink_version"},
				Description:   "Per-app Vector cron sink DSN (`SINK_TYPE://?SINK_OPTIONS`). Stored in Terraform state; prefer `vector_cron_sink_wo` for DSNs containing secrets.",
			},
			"vector_cron_sink_wo": {
				Type:          schema.TypeString,
				Optional:      true,
				Sensitive:     true,
				WriteOnly:     true,
				ValidateFunc:  validateLogsVectorSink,
				ConflictsWith: []string{"vector_cron_sink"},
				RequiredWith:  []string{"vector_cron_sink_version"},
				Description:   "Write-only per-app Vector cron sink DSN (`SINK_TYPE://?SINK_OPTIONS`); not saved in Terraform state (requires Terraform/OpenTofu >= 1.11).",
			},
			"vector_cron_sink_version": {
				Type:          schema.TypeInt,
				Optional:      true,
				ValidateFunc:  validation.IntAtLeast(1),
				ConflictsWith: []string{"vector_cron_sink"},
				RequiredWith:  []string{"vector_cron_sink_wo"},
				Description:   "Increment to apply an updated `vector_cron_sink_wo` DSN.",
			},
			"vector_cron_sink_fingerprint": {
				Type:        schema.TypeString,
				Computed:    true,
				Sensitive:   true,
				Description: "SHA-256 hex digest of the remote app `vector-cron-sink` DSN, or empty when unset.",
			},
			"rebuild_on_max_size_change": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     true,
				Description: "Whether to run `ps:rebuild <app>` when `max_size` is set, changed, or unset. Undeployed apps ignore non-zero rebuild status.",
			},
			"computed_max_size": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Effective app log max size (app setting or inherited global default).",
			},
			"computed_app_label_alias": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Effective app label alias (app setting or inherited global default).",
			},
		},
	}
}

func observeAppLogs(d *schema.ResourceData, app string, report dokkuLogsReport, warnSinkDrift, warnCronSinkDrift bool) diag.Diagnostics {
	var diags diag.Diagnostics
	for k, v := range map[string]interface{}{
		"app":                      app,
		"max_size":                 report.MaxSize,
		"app_label_alias":          report.AppLabelAlias,
		"computed_max_size":        report.ComputedMaxSize,
		"computed_app_label_alias": report.ComputedAppLabelAlias,
	} {
		if err := d.Set(k, v); err != nil {
			return diag.FromErr(err)
		}
	}
	diags = append(diags, observeLogsSink(d, vectorSinkSpec, report.VectorSink, warnSinkDrift)...)
	diags = append(diags, observeLogsSink(d, vectorCronSinkSpec, report.VectorCronSink, warnCronSinkDrift)...)
	return diags
}

func readAndObserveAppLogs(ctx context.Context, d *schema.ResourceData, run dokkuLogsRunner, app string, warnSinkDrift, warnCronSinkDrift bool) diag.Diagnostics {
	report, err := readAppLogsReport(ctx, run, app)
	if err != nil {
		return diag.FromErr(err)
	}
	return observeAppLogs(d, app, report, warnSinkDrift, warnCronSinkDrift)
}

func resourceAppLogsCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	return resourceAppLogsCreateWithRunner(ctx, d, newDokkuLogsRunner(m.(*goph.Client)))
}

func resourceAppLogsCreateWithRunner(ctx context.Context, d *schema.ResourceData, run dokkuLogsRunner) diag.Diagnostics {
	app := d.Get("app").(string)
	desiredSink, diags := resolveLogsSinkConfig(d, vectorSinkSpec)
	if diags.HasError() {
		return diags
	}
	desiredCronSink, diags := resolveLogsSinkConfig(d, vectorCronSinkSpec)
	if diags.HasError() {
		return diags
	}

	exists, err := checkDokkuAppExistsForLogs(ctx, run, app)
	if err != nil {
		return diag.FromErr(err)
	}
	if !exists {
		return diag.Errorf("Dokku app %q does not exist", app)
	}

	current, err := readAppLogsReport(ctx, run, app)
	if err != nil {
		return diag.FromErr(err)
	}

	desiredMaxSize := d.Get("max_size").(string)
	desiredAlias := d.Get("app_label_alias").(string)
	maxSizeChanged := desiredMaxSize != current.MaxSize

	if maxSizeChanged {
		if err := setAppLogProperty(ctx, run, app, "max-size", desiredMaxSize, false); err != nil {
			return diag.FromErr(err)
		}
	}
	if desiredAlias != current.AppLabelAlias {
		if err := setAppLogProperty(ctx, run, app, "app-label-alias", desiredAlias, false); err != nil {
			return diag.FromErr(err)
		}
	}
	if desiredSink != current.VectorSink {
		if err := setAppLogProperty(ctx, run, app, "vector-sink", desiredSink, true); err != nil {
			return diag.FromErr(err)
		}
	}
	if desiredCronSink != current.VectorCronSink {
		if err := setAppLogProperty(ctx, run, app, "vector-cron-sink", desiredCronSink, true); err != nil {
			return diag.FromErr(err)
		}
	}

	if maxSizeChanged && d.Get("rebuild_on_max_size_change").(bool) {
		if err := rebuildDokkuAppForLogs(ctx, run, app); err != nil {
			return diag.FromErr(err)
		}
	}

	d.SetId(app)
	if err := d.Set(vectorSinkSpec.fpAttr, logsSinkFingerprint(desiredSink)); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set(vectorCronSinkSpec.fpAttr, logsSinkFingerprint(desiredCronSink)); err != nil {
		return diag.FromErr(err)
	}
	return readAndObserveAppLogs(ctx, d, run, app, false, false)
}

func resourceAppLogsRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	return resourceAppLogsReadWithRunner(ctx, d, newDokkuLogsRunner(m.(*goph.Client)))
}

func resourceAppLogsReadWithRunner(ctx context.Context, d *schema.ResourceData, run dokkuLogsRunner) diag.Diagnostics {
	app := d.Id()
	if app == "" {
		app = d.Get("app").(string)
	}
	exists, err := checkDokkuAppExistsForLogs(ctx, run, app)
	if err != nil {
		return diag.FromErr(err)
	}
	if !exists {
		d.SetId("")
		return nil
	}
	return readAndObserveAppLogs(ctx, d, run, app, true, true)
}

func resourceAppLogsUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	return resourceAppLogsUpdateWithRunner(ctx, d, newDokkuLogsRunner(m.(*goph.Client)))
}

func resourceAppLogsUpdateWithRunner(ctx context.Context, d *schema.ResourceData, run dokkuLogsRunner) diag.Diagnostics {
	app := d.Id()
	sinkChanged := d.HasChange(vectorSinkSpec.plainAttr) || d.HasChange(vectorSinkSpec.versionAttr)
	cronSinkChanged := d.HasChange(vectorCronSinkSpec.plainAttr) || d.HasChange(vectorCronSinkSpec.versionAttr)

	var desiredSink, desiredCronSink string
	if sinkChanged {
		var diags diag.Diagnostics
		desiredSink, diags = resolveLogsSinkConfig(d, vectorSinkSpec)
		if diags.HasError() {
			return diags
		}
	}
	if cronSinkChanged {
		var diags diag.Diagnostics
		desiredCronSink, diags = resolveLogsSinkConfig(d, vectorCronSinkSpec)
		if diags.HasError() {
			return diags
		}
	}

	exists, err := checkDokkuAppExistsForLogs(ctx, run, app)
	if err != nil {
		return diag.FromErr(err)
	}
	if !exists {
		return diag.Errorf("Dokku app %q does not exist", app)
	}

	maxSizeChanged := d.HasChange("max_size")
	if maxSizeChanged {
		if err := setAppLogProperty(ctx, run, app, "max-size", d.Get("max_size").(string), false); err != nil {
			return diag.FromErr(err)
		}
	}
	if d.HasChange("app_label_alias") {
		if err := setAppLogProperty(ctx, run, app, "app-label-alias", d.Get("app_label_alias").(string), false); err != nil {
			return diag.FromErr(err)
		}
	}
	if sinkChanged {
		if err := setAppLogProperty(ctx, run, app, "vector-sink", desiredSink, true); err != nil {
			return diag.FromErr(err)
		}
		if err := d.Set(vectorSinkSpec.fpAttr, logsSinkFingerprint(desiredSink)); err != nil {
			return diag.FromErr(err)
		}
	}
	if cronSinkChanged {
		if err := setAppLogProperty(ctx, run, app, "vector-cron-sink", desiredCronSink, true); err != nil {
			return diag.FromErr(err)
		}
		if err := d.Set(vectorCronSinkSpec.fpAttr, logsSinkFingerprint(desiredCronSink)); err != nil {
			return diag.FromErr(err)
		}
	}

	if maxSizeChanged && d.Get("rebuild_on_max_size_change").(bool) {
		if err := rebuildDokkuAppForLogs(ctx, run, app); err != nil {
			return diag.FromErr(err)
		}
	}

	return readAndObserveAppLogs(ctx, d, run, app, !sinkChanged, !cronSinkChanged)
}

func resourceAppLogsDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	return resourceAppLogsDeleteWithRunner(ctx, d, newDokkuLogsRunner(m.(*goph.Client)))
}

func resourceAppLogsDeleteWithRunner(ctx context.Context, d *schema.ResourceData, run dokkuLogsRunner) diag.Diagnostics {
	app := d.Id()
	exists, err := checkDokkuAppExistsForLogs(ctx, run, app)
	if err != nil {
		return diag.FromErr(err)
	}
	if !exists {
		d.SetId("")
		return nil
	}

	wasMaxSizeSet := d.Get("max_size").(string) != ""
	for _, prop := range []struct {
		name      string
		sensitive bool
	}{
		{"max-size", false},
		{"app-label-alias", false},
		{"vector-sink", true},
		{"vector-cron-sink", true},
	} {
		if err := setAppLogProperty(ctx, run, app, prop.name, "", prop.sensitive); err != nil {
			return diag.FromErr(err)
		}
	}

	if wasMaxSizeSet && d.Get("rebuild_on_max_size_change").(bool) {
		if err := rebuildDokkuAppForLogs(ctx, run, app); err != nil {
			return diag.FromErr(err)
		}
	}

	d.SetId("")
	return nil
}

func resourceAppLogsImport(ctx context.Context, d *schema.ResourceData, m interface{}) ([]*schema.ResourceData, error) {
	return resourceAppLogsImportWithRunner(ctx, d, newDokkuLogsRunner(m.(*goph.Client)))
}

func resourceAppLogsImportWithRunner(ctx context.Context, d *schema.ResourceData, run dokkuLogsRunner) ([]*schema.ResourceData, error) {
	app := d.Id()
	if err := validateLogsAppNameValue(app); err != nil {
		return nil, fmt.Errorf("invalid app name %q: %w", app, err)
	}
	exists, err := checkDokkuAppExistsForLogs(ctx, run, app)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("Dokku app %q does not exist", app)
	}
	if err := d.Set("rebuild_on_max_size_change", true); err != nil {
		return nil, err
	}
	diags := readAndObserveAppLogs(ctx, d, run, app, false, false)
	if diags.HasError() {
		return nil, fmt.Errorf("%s", diags[0].Summary)
	}
	return []*schema.ResourceData{d}, nil
}
