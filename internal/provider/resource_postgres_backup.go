package provider

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/melbahja/goph"
)

func validateBackupValue(kind string) schema.SchemaValidateFunc {
	return func(value interface{}, key string) ([]string, []error) {
		v := value.(string)
		invalid := false
		switch kind {
		case "service":
			invalid = !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`).MatchString(v)
		case "schedule":
			invalid = strings.TrimSpace(v) == "" || strings.ContainsAny(v, "\x00\r\n")
		case "bucket":
			invalid = v == "" || strings.HasPrefix(strings.ToLower(v), "s3://") || strings.ContainsAny(v, " \t\r\n\x00")
		case "endpoint":
			if v != "" {
				u, err := url.Parse(v)
				invalid = err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || strings.ContainsAny(v, " \t\r\n\x00")
			}
		case "text":
			invalid = strings.TrimSpace(v) == "" || strings.ContainsAny(v, " \t\r\n\x00")
		}
		if invalid {
			return nil, []error{fmt.Errorf("invalid %s", key)}
		}
		return nil, nil
	}
}

func resourcePostgresBackup() *schema.Resource {
	return &schema.Resource{
		Description:   "Manages the default scheduled S3 backup for an existing Dokku Postgres service. Requires a postgres plugin that reports backup schedules and authentication fingerprints.",
		CreateContext: resourcePostgresBackupCreate,
		ReadContext:   resourcePostgresBackupRead,
		UpdateContext: resourcePostgresBackupUpdate,
		DeleteContext: resourcePostgresBackupDelete,
		CustomizeDiff: func(ctx context.Context, d *schema.ResourceDiff, m interface{}) error {
			if d.Get("schedule_sync_pending").(bool) {
				return d.SetNew("schedule_sync_pending", false)
			}
			return nil
		},
		Importer: &schema.ResourceImporter{StateContext: resourcePostgresBackupImport},
		Schema: map[string]*schema.Schema{
			"service_name":               {Type: schema.TypeString, Required: true, ForceNew: true, ValidateFunc: validateBackupValue("service"), Description: "Name of the existing Dokku Postgres service."},
			"bucket":                     {Type: schema.TypeString, Required: true, ValidateFunc: validateBackupValue("bucket"), Description: "S3 bucket name, optionally followed by a path prefix; omit the s3:// scheme."},
			"schedule":                   {Type: schema.TypeString, Required: true, ValidateFunc: validateBackupValue("schedule"), Description: "Cron schedule or descriptor, interpreted in the server's cron timezone."},
			"endpoint_url":               {Type: schema.TypeString, Optional: true, Default: "", ValidateFunc: validateBackupValue("endpoint"), Description: "Optional HTTP or HTTPS endpoint for S3-compatible storage."},
			"region":                     {Type: schema.TypeString, Optional: true, Default: "us-east-1", ValidateFunc: validateBackupValue("text"), Description: "S3 region used for backup authentication."},
			"signature_version":          {Type: schema.TypeString, Optional: true, Default: "s3v4", ValidateFunc: validateBackupValue("text"), Description: "S3 signature version used for backup authentication."},
			"access_key_id_wo":           {Type: schema.TypeString, Required: true, Sensitive: true, WriteOnly: true, Description: "Write-only S3 access key ID; not saved in Terraform state."},
			"secret_access_key_wo":       {Type: schema.TypeString, Required: true, Sensitive: true, WriteOnly: true, Description: "Write-only S3 secret access key; not saved in Terraform state."},
			"credentials_version":        {Type: schema.TypeInt, Optional: true, Default: 1, ValidateFunc: validation.IntAtLeast(1), Description: "Increment to apply rotated write-only credentials."},
			"schedule_sync_pending":      {Type: schema.TypeBool, Computed: true, Description: "Whether a failed schedule update requires retry."},
			"owned_schedule":             {Type: schema.TypeString, Computed: true, Description: "Last schedule applied or imported by this resource for ownership checks."},
			"owned_bucket":               {Type: schema.TypeString, Computed: true, Description: "Last bucket applied or imported by this resource for ownership checks."},
			"deauth_pending":             {Type: schema.TypeBool, Computed: true, Description: "Whether a credential removal may need recovery."},
			"auth_pending":               {Type: schema.TypeBool, Computed: true, Description: "Whether a credential operation may need recovery."},
			"owned_access_fingerprint":   {Type: schema.TypeString, Computed: true, Sensitive: true, Description: "Fingerprint of the last owned access key."},
			"owned_secret_fingerprint":   {Type: schema.TypeString, Computed: true, Sensitive: true, Description: "Fingerprint of the last owned secret key."},
			"pending_access_fingerprint": {Type: schema.TypeString, Computed: true, Sensitive: true, Description: "Fingerprint of the access key in an incomplete operation."},
			"pending_secret_fingerprint": {Type: schema.TypeString, Computed: true, Sensitive: true, Description: "Fingerprint of the secret key in an incomplete operation."},
			"auth_fingerprint":           {Type: schema.TypeString, Computed: true, Sensitive: true, Description: "Fingerprint of credentials last applied or imported by this resource for ownership checks."},
		},
	}
}

func backupCredentials(d *schema.ResourceData) (string, string, diag.Diagnostics) {
	values := make([]string, 0, 2)
	for _, name := range []string{"access_key_id_wo", "secret_access_key_wo"} {
		value, diagnostics := d.GetRawConfigAt(cty.GetAttrPath(name))
		if diagnostics.HasError() {
			return "", "", diagnostics
		}
		if !value.IsKnown() || value.IsNull() || value.Type() != cty.String || value.AsString() == "" {
			return "", "", diag.Errorf("%s must be a known, nonempty value when applying backup credentials", name)
		}
		values = append(values, value.AsString())
	}
	return values[0], values[1], nil
}

func backupObserve(d *schema.ResourceData, state postgresBackupState) error {
	for key, value := range map[string]interface{}{
		"service_name": d.Id(), "bucket": state.Bucket, "schedule": state.Schedule,
		"region": state.Region, "signature_version": state.SignatureVersion, "endpoint_url": state.Endpoint,
	} {
		if err := d.Set(key, value); err != nil {
			return fmt.Errorf("set backup %s: %w", key, err)
		}
	}
	return nil
}

func backupRecordOwnership(d *schema.ResourceData, state postgresBackupState) error {
	for key, value := range map[string]string{
		"owned_schedule": state.Schedule, "owned_bucket": state.Bucket, "auth_fingerprint": state.AuthFingerprint, "owned_access_fingerprint": state.AccessKeyFingerprint, "owned_secret_fingerprint": state.SecretKeyFingerprint,
	} {
		if err := d.Set(key, value); err != nil {
			return fmt.Errorf("record backup %s: %w", key, err)
		}
	}
	return nil
}

func backupAuthCleared(state postgresBackupState) bool {
	return !state.Authenticated && !state.AuthPartial && state.Region == "" && state.SignatureVersion == "" && state.Endpoint == ""
}

func backupPendingMatches(d *schema.ResourceData, state postgresBackupState) bool {
	if !d.Get("auth_pending").(bool) {
		return false
	}
	access, secret := d.Get("pending_access_fingerprint").(string), d.Get("pending_secret_fingerprint").(string)
	if state.AccessKeyFingerprint == "" && state.SecretKeyFingerprint == "" {
		return d.Get("deauth_pending").(bool)
	}
	oldAccess, oldSecret := d.Get("owned_access_fingerprint").(string), d.Get("owned_secret_fingerprint").(string)
	return (state.AccessKeyFingerprint == "" || state.AccessKeyFingerprint == access || (oldAccess != "" && state.AccessKeyFingerprint == oldAccess)) && (state.SecretKeyFingerprint == "" || state.SecretKeyFingerprint == secret || (oldSecret != "" && state.SecretKeyFingerprint == oldSecret))
}

func resourcePostgresBackupCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	runner := newPostgresBackupRunner(m.(*goph.Client))
	service := d.Get("service_name").(string)
	current, err := readPostgresBackup(ctx, runner, service)
	if err != nil {
		return diag.FromErr(err)
	}
	if !current.ServiceExists {
		return diag.Errorf("Postgres service %q does not exist", service)
	}
	if current.Scheduled || current.Authenticated || current.AuthPartial || current.Region != "" || current.SignatureVersion != "" || current.Endpoint != "" {
		return diag.Errorf("backup configuration already exists for %q; import it instead of overwriting it", service)
	}
	access, secret, diagnostics := backupCredentials(d)
	if diagnostics.HasError() {
		return diagnostics
	}
	requestedSchedule, requestedBucket := d.Get("schedule").(string), d.Get("bucket").(string)
	fingerprint := postgresBackupFingerprint(access, secret)
	d.SetId(service)
	_ = d.Set("auth_pending", true)
	_ = d.Set("pending_access_fingerprint", postgresBackupValueFingerprint(access))
	_ = d.Set("pending_secret_fingerprint", postgresBackupValueFingerprint(secret))
	recoverCreate := func(cause error) diag.Diagnostics {
		observed, readErr := readPostgresBackup(ctx, runner, service)
		if readErr == nil {
			if observed.AuthPartial && !observed.Scheduled && backupPendingMatches(d, observed) {
				_ = d.Set("deauth_pending", true)
				if cleanupErr := deauthPostgresBackup(ctx, runner, service); cleanupErr == nil {
					if cleaned, checkErr := readPostgresBackup(ctx, runner, service); checkErr == nil && backupAuthCleared(cleaned) && !cleaned.Scheduled {
						d.SetId("")
					}
				}
			} else if !observed.Authenticated && !observed.AuthPartial && !observed.Scheduled {
				if backupAuthCleared(observed) {
					d.SetId("")
				} else if backupPendingMatches(d, observed) {
					_ = d.Set("deauth_pending", true)
					if cleanupErr := deauthPostgresBackup(ctx, runner, service); cleanupErr == nil {
						if cleaned, checkErr := readPostgresBackup(ctx, runner, service); checkErr == nil && backupAuthCleared(cleaned) && !cleaned.Scheduled {
							d.SetId("")
						}
					}
				}
			} else if observed.Authenticated && observed.AuthFingerprint == fingerprint && (!observed.Scheduled || (observed.Schedule == requestedSchedule && observed.Bucket == requestedBucket)) {
				_ = backupRecordOwnership(d, observed)
				_ = d.Set("auth_pending", false)
				_ = backupObserve(d, observed)
			}
		}
		return diag.FromErr(cause)
	}
	if err := authPostgresBackup(ctx, runner, service, access, secret, d.Get("region").(string), d.Get("signature_version").(string), d.Get("endpoint_url").(string)); err != nil {
		return recoverCreate(err)
	}
	current, err = readPostgresBackup(ctx, runner, service)
	if err != nil {
		return recoverCreate(err)
	}
	if !current.Authenticated || current.AuthFingerprint != postgresBackupFingerprint(access, secret) || current.Region != d.Get("region").(string) || current.SignatureVersion != d.Get("signature_version").(string) || current.Endpoint != d.Get("endpoint_url").(string) {
		return recoverCreate(fmt.Errorf("backup authentication verification failed for %q", service))
	}
	if err := backupRecordOwnership(d, current); err != nil {
		return diag.FromErr(err)
	}
	_ = d.Set("auth_pending", false)
	if err := schedulePostgresBackup(ctx, runner, service, d.Get("schedule").(string), d.Get("bucket").(string)); err != nil {
		return recoverCreate(err)
	}
	current, err = readPostgresBackup(ctx, runner, service)
	if err != nil {
		return recoverCreate(err)
	}
	if !current.Scheduled || current.Schedule != d.Get("schedule").(string) || current.Bucket != d.Get("bucket").(string) {
		return recoverCreate(fmt.Errorf("backup schedule verification failed for %q", service))
	}
	if err := backupRecordOwnership(d, current); err != nil {
		return diag.FromErr(err)
	}
	if err := backupObserve(d, current); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourcePostgresBackupRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	state, err := readPostgresBackup(ctx, newPostgresBackupRunner(m.(*goph.Client)), d.Id())
	if err != nil {
		return diag.FromErr(err)
	}
	if !state.ServiceExists {
		d.SetId("")
		return nil
	}
	if err := backupObserve(d, state); err != nil {
		return diag.FromErr(err)
	}
	if state.AuthFingerprint != d.Get("auth_fingerprint").(string) {
		return diag.Diagnostics{{Severity: diag.Warning, Summary: "Backup authentication changed outside Terraform", Detail: "Increment credentials_version to explicitly rotate the credentials. The resource will not adopt the externally changed authentication."}}
	}
	return nil
}

func resourcePostgresBackupUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	runner := newPostgresBackupRunner(m.(*goph.Client))
	service := d.Id()
	current, err := readPostgresBackup(ctx, runner, service)
	if err != nil {
		return diag.FromErr(err)
	}
	if !current.ServiceExists {
		return diag.Errorf("Postgres service %q does not exist", service)
	}
	if d.HasChanges("region", "signature_version", "endpoint_url") && !d.HasChange("credentials_version") && current.AuthFingerprint != d.Get("auth_fingerprint").(string) {
		return diag.Errorf("backup authentication changed outside Terraform; increment credentials_version to explicitly rotate credentials")
	}
	requestedSchedule, requestedBucket := d.Get("schedule").(string), d.Get("bucket").(string)
	requestedRegion, requestedSignature, requestedEndpoint := d.Get("region").(string), d.Get("signature_version").(string), d.Get("endpoint_url").(string)
	oldVersion, _ := d.GetChange("credentials_version")
	oldPending, _ := d.GetChange("schedule_sync_pending")
	oldFingerprint := d.Get("auth_fingerprint").(string)
	targetFingerprint := ""
	scheduleAttempted := false
	d.Partial(true)
	recoverUpdate := func(cause error) diag.Diagnostics {
		observed, readErr := readPostgresBackup(ctx, runner, service)
		if readErr != nil {
			if scheduleAttempted || oldPending.(bool) {
				d.Partial(false)
				for _, key := range []string{"bucket", "schedule", "region", "signature_version", "endpoint_url", "credentials_version"} {
					old, _ := d.GetChange(key)
					_ = d.Set(key, old)
				}
				_ = d.Set("schedule_sync_pending", true)
			}
			return diag.FromErr(cause)
		}
		scheduleOwned := (observed.Schedule == d.Get("owned_schedule").(string) && observed.Bucket == d.Get("owned_bucket").(string)) || (scheduleAttempted && observed.Schedule == requestedSchedule && observed.Bucket == requestedBucket)
		pendingOwned := targetFingerprint != "" && backupPendingMatches(d, observed)
		authApplied := targetFingerprint != "" && observed.Authenticated && observed.AuthFingerprint == targetFingerprint && observed.Region == requestedRegion && observed.SignatureVersion == requestedSignature && observed.Endpoint == requestedEndpoint
		if !scheduleOwned || (observed.AuthFingerprint != oldFingerprint && !authApplied && !pendingOwned) {
			if scheduleAttempted || oldPending.(bool) {
				d.Partial(false)
				for _, key := range []string{"bucket", "schedule", "region", "signature_version", "endpoint_url", "credentials_version"} {
					old, _ := d.GetChange(key)
					_ = d.Set(key, old)
				}
				_ = d.Set("schedule_sync_pending", true)
			}
			return diag.FromErr(cause)
		}
		d.Partial(false)
		if scheduleAttempted || oldPending.(bool) {
			_ = d.Set("schedule_sync_pending", true)
		}
		if authApplied {
			_ = d.Set("auth_fingerprint", observed.AuthFingerprint)
			_ = d.Set("owned_access_fingerprint", observed.AccessKeyFingerprint)
			_ = d.Set("owned_secret_fingerprint", observed.SecretKeyFingerprint)
			_ = d.Set("auth_pending", false)
		} else {
			_ = d.Set("auth_fingerprint", oldFingerprint)
			_ = d.Set("credentials_version", oldVersion)
		}
		_ = d.Set("owned_schedule", observed.Schedule)
		_ = d.Set("owned_bucket", observed.Bucket)
		_ = backupObserve(d, observed)
		return diag.FromErr(cause)
	}
	if d.HasChanges("credentials_version", "region", "signature_version", "endpoint_url") {
		access, secret, diagnostics := backupCredentials(d)
		if diagnostics.HasError() {
			return diagnostics
		}
		targetFingerprint = postgresBackupFingerprint(access, secret)
		_ = d.Set("auth_pending", true)
		_ = d.Set("pending_access_fingerprint", postgresBackupValueFingerprint(access))
		_ = d.Set("pending_secret_fingerprint", postgresBackupValueFingerprint(secret))
		if err := authPostgresBackup(ctx, runner, service, access, secret, d.Get("region").(string), d.Get("signature_version").(string), d.Get("endpoint_url").(string)); err != nil {
			return recoverUpdate(err)
		}
		current, err = readPostgresBackup(ctx, runner, service)
		if err != nil {
			return recoverUpdate(err)
		}
		if !current.Authenticated || current.AuthFingerprint != postgresBackupFingerprint(access, secret) || current.Region != d.Get("region").(string) || current.SignatureVersion != d.Get("signature_version").(string) || current.Endpoint != d.Get("endpoint_url").(string) {
			return recoverUpdate(fmt.Errorf("backup authentication verification failed for %q", service))
		}
		if err := d.Set("auth_fingerprint", current.AuthFingerprint); err != nil {
			return diag.FromErr(err)
		}
		_ = d.Set("owned_access_fingerprint", current.AccessKeyFingerprint)
		_ = d.Set("owned_secret_fingerprint", current.SecretKeyFingerprint)
		_ = d.Set("auth_pending", false)
	}
	if d.HasChanges("schedule", "bucket", "schedule_sync_pending") {
		scheduleAttempted = true
		if err := schedulePostgresBackup(ctx, runner, service, d.Get("schedule").(string), d.Get("bucket").(string)); err != nil {
			return recoverUpdate(err)
		}
		current, err = readPostgresBackup(ctx, runner, service)
		if err != nil {
			return recoverUpdate(err)
		}
		if !current.Scheduled || current.Schedule != d.Get("schedule").(string) || current.Bucket != d.Get("bucket").(string) {
			return recoverUpdate(fmt.Errorf("backup schedule verification failed for %q", service))
		}
	}
	if scheduleAttempted {
		if err := d.Set("schedule_sync_pending", false); err != nil {
			return diag.FromErr(err)
		}
		if err := d.Set("owned_schedule", current.Schedule); err != nil {
			return diag.FromErr(err)
		}
		if err := d.Set("owned_bucket", current.Bucket); err != nil {
			return diag.FromErr(err)
		}
	}
	if err := backupObserve(d, current); err != nil {
		return diag.FromErr(err)
	}
	d.Partial(false)
	return nil
}

func resourcePostgresBackupDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	runner := newPostgresBackupRunner(m.(*goph.Client))
	state, err := readPostgresBackup(ctx, runner, d.Id())
	if err != nil {
		return diag.FromErr(err)
	}
	if !state.ServiceExists {
		d.SetId("")
		return nil
	}
	if state.AuthPartial && !backupPendingMatches(d, state) {
		return diag.Errorf("partial backup authentication is not owned by this resource; refusing to remove it")
	}
	if !state.Scheduled && backupAuthCleared(state) {
		d.SetId("")
		return nil
	}
	if !state.Authenticated && !state.AuthPartial && !backupPendingMatches(d, state) {
		return diag.Errorf("backup authentication metadata remains without owned credentials; refusing to remove it")
	}
	if state.Scheduled && (state.Schedule != d.Get("owned_schedule").(string) || state.Bucket != d.Get("owned_bucket").(string)) {
		return diag.Errorf("backup schedule changed outside Terraform; refusing to remove it")
	}
	if state.Authenticated && state.AuthFingerprint != d.Get("auth_fingerprint").(string) && !backupPendingMatches(d, state) {
		return diag.Errorf("backup authentication changed outside Terraform; refusing to remove it")
	}
	if state.Scheduled {
		if err := unschedulePostgresBackup(ctx, runner, d.Id()); err != nil {
			return diag.FromErr(err)
		}
		state, err = readPostgresBackup(ctx, runner, d.Id())
		if err != nil {
			return diag.FromErr(err)
		}
		if state.Scheduled {
			return diag.Errorf("backup schedule removal could not be verified")
		}
		if state.AuthPartial && !backupPendingMatches(d, state) {
			return diag.Errorf("partial backup authentication changed during removal; refusing to remove it")
		}
		if !state.Authenticated && !state.AuthPartial && !backupAuthCleared(state) && !backupPendingMatches(d, state) {
			return diag.Errorf("backup authentication metadata changed during removal; refusing to remove it")
		}
		if state.Authenticated && state.AuthFingerprint != d.Get("auth_fingerprint").(string) && !backupPendingMatches(d, state) {
			return diag.Errorf("backup authentication changed during removal; refusing to remove it")
		}
	}
	if !backupAuthCleared(state) {
		if !d.Get("auth_pending").(bool) {
			_ = d.Set("auth_pending", true)
			_ = d.Set("pending_access_fingerprint", d.Get("owned_access_fingerprint").(string))
			_ = d.Set("pending_secret_fingerprint", d.Get("owned_secret_fingerprint").(string))
		}
		_ = d.Set("deauth_pending", true)
		if err := deauthPostgresBackup(ctx, runner, d.Id()); err != nil {
			return diag.FromErr(err)
		}
		state, err = readPostgresBackup(ctx, runner, d.Id())
		if err != nil {
			return diag.FromErr(err)
		}
		if state.Scheduled || !backupAuthCleared(state) {
			return diag.Errorf("backup authentication removal could not be verified")
		}
	}
	d.SetId("")
	return nil
}

func resourcePostgresBackupImport(ctx context.Context, d *schema.ResourceData, m interface{}) ([]*schema.ResourceData, error) {
	state, err := readPostgresBackup(ctx, newPostgresBackupRunner(m.(*goph.Client)), d.Id())
	if err != nil {
		return nil, err
	}
	if !state.ServiceExists || !state.Scheduled || !state.Authenticated {
		return nil, fmt.Errorf("service %q does not have a scheduled backup with configured authentication", d.Id())
	}
	if err := backupRecordOwnership(d, state); err != nil {
		return nil, err
	}
	if err := backupObserve(d, state); err != nil {
		return nil, err
	}
	// There is no remote credential revision to read. Establish the documented
	// baseline so importing with the default does not reapply credentials.
	if err := d.Set("credentials_version", 1); err != nil {
		return nil, err
	}
	return []*schema.ResourceData{d}, nil
}
