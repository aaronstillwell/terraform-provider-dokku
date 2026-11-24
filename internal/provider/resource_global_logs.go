package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/melbahja/goph"
)

func resourceGlobalLogs() *schema.Resource {
	return &schema.Resource{
		Description:   "Manages global Dokku log and Vector configuration (`dokku logs:set --global`, `logs:vector-start`, `logs:vector-stop`). Singleton resource with ID `global`.",
		CreateContext: resourceGlobalLogsCreate,
		ReadContext:   resourceGlobalLogsRead,
		UpdateContext: resourceGlobalLogsUpdate,
		DeleteContext: resourceGlobalLogsDelete,
		CustomizeDiff: func(ctx context.Context, d *schema.ResourceDiff, m interface{}) error {
			if d.GetRawConfig().IsNull() {
				return nil
			}
			val, diags := d.GetRawConfigAt(cty.GetAttrPath("vector_running"))
			if diags.HasError() {
				return fmt.Errorf("%s", diags[0].Summary)
			}
			configured := !val.IsNull()
			if d.Get("owned_vector_running").(bool) != configured {
				return d.SetNew("owned_vector_running", configured)
			}
			return nil
		},
		Importer: &schema.ResourceImporter{StateContext: resourceGlobalLogsImport},
		Schema: map[string]*schema.Schema{
			"max_size": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validateLogsMaxSize,
				Description:  "Global container log max size (e.g. `10m`, `1g`, or `unlimited`). Unset when omitted.",
			},
			"app_label_alias": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validateLogsAppLabelAlias,
				Description:  "Global Vector label alias for the Dokku app name. Unset when omitted.",
			},
			"vector_image": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validateLogsVectorImage,
				Description:  "Docker image used for the global Dokku Vector container. Unset when omitted.",
			},
			"vector_networks": {
				Type:        schema.TypeSet,
				Optional:    true,
				Description: "Docker networks attached to the global Vector container (`bridge` is disallowed). Unset when omitted.",
				Elem: &schema.Schema{
					Type:         schema.TypeString,
					ValidateFunc: validateLogsVectorNetworkItem,
				},
			},
			"vector_running": {
				Type:        schema.TypeBool,
				Optional:    true,
				Computed:    true,
				Description: "Whether the global Dokku Vector container should be running. Omit to leave an externally managed Vector container untouched.",
			},
			"owned_vector_running": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Internal marker tracking whether `vector_running` is explicitly managed by this resource.",
			},
			"vector_sink": {
				Type:          schema.TypeString,
				Optional:      true,
				Sensitive:     true,
				ValidateFunc:  validateLogsVectorSink,
				ConflictsWith: []string{"vector_sink_wo", "vector_sink_version"},
				Description:   "Global Vector sink DSN (`SINK_TYPE://?SINK_OPTIONS`). Stored in Terraform state; prefer `vector_sink_wo` for DSNs containing secrets.",
			},
			"vector_sink_wo": {
				Type:          schema.TypeString,
				Optional:      true,
				Sensitive:     true,
				WriteOnly:     true,
				ValidateFunc:  validateLogsVectorSink,
				ConflictsWith: []string{"vector_sink"},
				RequiredWith:  []string{"vector_sink_version"},
				Description:   "Write-only global Vector sink DSN (`SINK_TYPE://?SINK_OPTIONS`); not saved in Terraform state (requires Terraform/OpenTofu >= 1.11).",
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
				Description: "SHA-256 hex digest of the remote global `vector-sink` DSN, or empty when unset.",
			},
			"vector_cron_sink": {
				Type:          schema.TypeString,
				Optional:      true,
				Sensitive:     true,
				ValidateFunc:  validateLogsVectorSink,
				ConflictsWith: []string{"vector_cron_sink_wo", "vector_cron_sink_version"},
				Description:   "Global Vector cron sink DSN (`SINK_TYPE://?SINK_OPTIONS`). Stored in Terraform state; prefer `vector_cron_sink_wo` for DSNs containing secrets.",
			},
			"vector_cron_sink_wo": {
				Type:          schema.TypeString,
				Optional:      true,
				Sensitive:     true,
				WriteOnly:     true,
				ValidateFunc:  validateLogsVectorSink,
				ConflictsWith: []string{"vector_cron_sink"},
				RequiredWith:  []string{"vector_cron_sink_version"},
				Description:   "Write-only global Vector cron sink DSN (`SINK_TYPE://?SINK_OPTIONS`); not saved in Terraform state (requires Terraform/OpenTofu >= 1.11).",
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
				Description: "SHA-256 hex digest of the remote global `vector-cron-sink` DSN, or empty when unset.",
			},
			"computed_max_size": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Effective global log max size reported by Dokku.",
			},
			"computed_app_label_alias": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Effective global app label alias reported by Dokku.",
			},
			"computed_vector_image": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Effective Vector Docker image reported by Dokku.",
			},
			"computed_vector_networks": {
				Type:        schema.TypeSet,
				Computed:    true,
				Description: "Effective Vector Docker networks reported by Dokku.",
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
		},
	}
}

func isVectorRunningConfigured(d *schema.ResourceData) (bool, diag.Diagnostics) {
	if d.GetRawConfig().IsNull() {
		return false, nil
	}
	val, diags := d.GetRawConfigAt(cty.GetAttrPath("vector_running"))
	if diags.HasError() {
		return false, diags
	}
	return !val.IsNull(), nil
}

func reconcileGlobalVectorRunning(
	ctx context.Context,
	run dokkuLogsRunner,
	owned bool,
	desiredRunning bool,
	currentRunning bool,
	imageChanged bool,
	networksChanged bool,
) error {
	if owned {
		if desiredRunning {
			if !currentRunning {
				return startLogsVector(ctx, run)
			}
			if imageChanged || networksChanged {
				if err := stopLogsVector(ctx, run); err != nil {
					return err
				}
				return startLogsVector(ctx, run)
			}
			return nil
		}
		if currentRunning {
			return stopLogsVector(ctx, run)
		}
		return nil
	}

	if currentRunning && (imageChanged || networksChanged) {
		if err := stopLogsVector(ctx, run); err != nil {
			return err
		}
		return startLogsVector(ctx, run)
	}
	return nil
}

func observeGlobalLogs(d *schema.ResourceData, report dokkuLogsReport, running bool, warnOnDrift bool) diag.Diagnostics {
	var diags diag.Diagnostics
	for k, v := range map[string]interface{}{
		"max_size":                 report.GlobalMaxSize,
		"app_label_alias":          report.GlobalAppLabelAlias,
		"vector_image":             report.GlobalVectorImage,
		"vector_networks":          report.GlobalVectorNetworks,
		"vector_running":           running,
		"computed_max_size":        report.ComputedMaxSize,
		"computed_app_label_alias": report.ComputedAppLabelAlias,
		"computed_vector_image":    report.ComputedVectorImage,
		"computed_vector_networks": report.ComputedVectorNetworks,
	} {
		if err := d.Set(k, v); err != nil {
			return diag.FromErr(err)
		}
	}
	diags = append(diags, observeLogsSink(d, vectorSinkSpec, report.GlobalVectorSink, warnOnDrift)...)
	diags = append(diags, observeLogsSink(d, vectorCronSinkSpec, report.GlobalVectorCronSink, warnOnDrift)...)
	return diags
}

func readAndObserveGlobalLogs(ctx context.Context, d *schema.ResourceData, run dokkuLogsRunner, warnOnDrift bool) diag.Diagnostics {
	report, err := readGlobalLogsReport(ctx, run)
	if err != nil {
		return diag.FromErr(err)
	}
	running, err := readLogsVectorRunning(ctx, run)
	if err != nil {
		return diag.FromErr(err)
	}
	return observeGlobalLogs(d, report, running, warnOnDrift)
}

func resourceGlobalLogsCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	return resourceGlobalLogsCreateWithRunner(ctx, d, newDokkuLogsRunner(m.(*goph.Client)))
}

func resourceGlobalLogsCreateWithRunner(ctx context.Context, d *schema.ResourceData, run dokkuLogsRunner) diag.Diagnostics {
	desiredSink, diags := resolveLogsSinkConfig(d, vectorSinkSpec)
	if diags.HasError() {
		return diags
	}
	desiredCronSink, diags := resolveLogsSinkConfig(d, vectorCronSinkSpec)
	if diags.HasError() {
		return diags
	}
	ownedRunning, diags := isVectorRunningConfigured(d)
	if diags.HasError() {
		return diags
	}

	currentReport, err := readGlobalLogsReport(ctx, run)
	if err != nil {
		return diag.FromErr(err)
	}
	currentRunning, err := readLogsVectorRunning(ctx, run)
	if err != nil {
		return diag.FromErr(err)
	}

	desiredMaxSize := d.Get("max_size").(string)
	desiredAlias := d.Get("app_label_alias").(string)
	desiredImage := d.Get("vector_image").(string)
	desiredNetworks := normalizeLogsNetworkSet(d.Get("vector_networks").(*schema.Set))
	desiredNetworksJoined := strings.Join(desiredNetworks, ",")

	imageChanged := desiredImage != currentReport.GlobalVectorImage
	networksChanged := !stringSlicesEqual(desiredNetworks, currentReport.GlobalVectorNetworks)

	if desiredMaxSize != currentReport.GlobalMaxSize {
		if err := setGlobalLogProperty(ctx, run, "max-size", desiredMaxSize, false); err != nil {
			return diag.FromErr(err)
		}
	}
	if desiredAlias != currentReport.GlobalAppLabelAlias {
		if err := setGlobalLogProperty(ctx, run, "app-label-alias", desiredAlias, false); err != nil {
			return diag.FromErr(err)
		}
	}
	if imageChanged {
		if err := setGlobalLogProperty(ctx, run, "vector-image", desiredImage, false); err != nil {
			return diag.FromErr(err)
		}
	}
	if networksChanged {
		if err := setGlobalLogProperty(ctx, run, "vector-networks", desiredNetworksJoined, false); err != nil {
			return diag.FromErr(err)
		}
	}
	if desiredSink != currentReport.GlobalVectorSink {
		if err := setGlobalLogProperty(ctx, run, "vector-sink", desiredSink, true); err != nil {
			return diag.FromErr(err)
		}
	}
	if desiredCronSink != currentReport.GlobalVectorCronSink {
		if err := setGlobalLogProperty(ctx, run, "vector-cron-sink", desiredCronSink, true); err != nil {
			return diag.FromErr(err)
		}
	}

	desiredRunning := d.Get("vector_running").(bool)
	if err := reconcileGlobalVectorRunning(ctx, run, ownedRunning, desiredRunning, currentRunning, imageChanged, networksChanged); err != nil {
		return diag.FromErr(err)
	}

	d.SetId("global")
	if err := d.Set("owned_vector_running", ownedRunning); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set(vectorSinkSpec.fpAttr, logsSinkFingerprint(desiredSink)); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set(vectorCronSinkSpec.fpAttr, logsSinkFingerprint(desiredCronSink)); err != nil {
		return diag.FromErr(err)
	}
	return readAndObserveGlobalLogs(ctx, d, run, false)
}

func resourceGlobalLogsRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	return resourceGlobalLogsReadWithRunner(ctx, d, newDokkuLogsRunner(m.(*goph.Client)))
}

func resourceGlobalLogsReadWithRunner(ctx context.Context, d *schema.ResourceData, run dokkuLogsRunner) diag.Diagnostics {
	return readAndObserveGlobalLogs(ctx, d, run, true)
}

func resourceGlobalLogsUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	return resourceGlobalLogsUpdateWithRunner(ctx, d, newDokkuLogsRunner(m.(*goph.Client)))
}

func resourceGlobalLogsUpdateWithRunner(ctx context.Context, d *schema.ResourceData, run dokkuLogsRunner) diag.Diagnostics {
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

	ownedRunning, diags := isVectorRunningConfigured(d)
	if diags.HasError() {
		return diags
	}
	currentRunning, err := readLogsVectorRunning(ctx, run)
	if err != nil {
		return diag.FromErr(err)
	}

	if d.HasChange("max_size") {
		if err := setGlobalLogProperty(ctx, run, "max-size", d.Get("max_size").(string), false); err != nil {
			return diag.FromErr(err)
		}
	}
	if d.HasChange("app_label_alias") {
		if err := setGlobalLogProperty(ctx, run, "app-label-alias", d.Get("app_label_alias").(string), false); err != nil {
			return diag.FromErr(err)
		}
	}
	imageChanged := d.HasChange("vector_image")
	if imageChanged {
		if err := setGlobalLogProperty(ctx, run, "vector-image", d.Get("vector_image").(string), false); err != nil {
			return diag.FromErr(err)
		}
	}
	networksChanged := d.HasChange("vector_networks")
	if networksChanged {
		joined := joinLogsNetworkSet(d.Get("vector_networks").(*schema.Set))
		if err := setGlobalLogProperty(ctx, run, "vector-networks", joined, false); err != nil {
			return diag.FromErr(err)
		}
	}
	if sinkChanged {
		if err := setGlobalLogProperty(ctx, run, "vector-sink", desiredSink, true); err != nil {
			return diag.FromErr(err)
		}
		if err := d.Set(vectorSinkSpec.fpAttr, logsSinkFingerprint(desiredSink)); err != nil {
			return diag.FromErr(err)
		}
	}
	if cronSinkChanged {
		if err := setGlobalLogProperty(ctx, run, "vector-cron-sink", desiredCronSink, true); err != nil {
			return diag.FromErr(err)
		}
		if err := d.Set(vectorCronSinkSpec.fpAttr, logsSinkFingerprint(desiredCronSink)); err != nil {
			return diag.FromErr(err)
		}
	}

	desiredRunning := d.Get("vector_running").(bool)
	if err := reconcileGlobalVectorRunning(ctx, run, ownedRunning, desiredRunning, currentRunning, imageChanged, networksChanged); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("owned_vector_running", ownedRunning); err != nil {
		return diag.FromErr(err)
	}

	return readAndObserveGlobalLogs(ctx, d, run, false)
}

func resourceGlobalLogsDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	return resourceGlobalLogsDeleteWithRunner(ctx, d, newDokkuLogsRunner(m.(*goph.Client)))
}

func resourceGlobalLogsDeleteWithRunner(ctx context.Context, d *schema.ResourceData, run dokkuLogsRunner) diag.Diagnostics {
	for _, prop := range []struct {
		name      string
		sensitive bool
	}{
		{"max-size", false},
		{"app-label-alias", false},
		{"vector-image", false},
		{"vector-networks", false},
		{"vector-sink", true},
		{"vector-cron-sink", true},
	} {
		if err := setGlobalLogProperty(ctx, run, prop.name, "", prop.sensitive); err != nil {
			return diag.FromErr(err)
		}
	}

	if d.Get("owned_vector_running").(bool) && d.Get("vector_running").(bool) {
		if err := stopLogsVector(ctx, run); err != nil {
			return diag.FromErr(err)
		}
	}

	d.SetId("")
	return nil
}

func resourceGlobalLogsImport(ctx context.Context, d *schema.ResourceData, m interface{}) ([]*schema.ResourceData, error) {
	return resourceGlobalLogsImportWithRunner(ctx, d, newDokkuLogsRunner(m.(*goph.Client)))
}

func resourceGlobalLogsImportWithRunner(ctx context.Context, d *schema.ResourceData, run dokkuLogsRunner) ([]*schema.ResourceData, error) {
	if d.Id() != "global" {
		return nil, fmt.Errorf("dokku_global_logs import ID must be \"global\", got %q", d.Id())
	}
	if err := d.Set("owned_vector_running", false); err != nil {
		return nil, err
	}
	diags := readAndObserveGlobalLogs(ctx, d, run, false)
	if diags.HasError() {
		return nil, fmt.Errorf("%s", diags[0].Summary)
	}
	return []*schema.ResourceData{d}, nil
}
