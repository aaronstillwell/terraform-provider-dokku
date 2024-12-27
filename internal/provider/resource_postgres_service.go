package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/melbahja/goph"
)

func resourcePostgresService() *schema.Resource {
	return &schema.Resource{
		Description: "Manages a Postgres database service, allowing for creation and configuration of Postgres databases. Requires the Postgres Dokku plugin to be installed.",
		CreateContext: resourcePgCreate,
		ReadContext:   resourcePgRead,
		UpdateContext: resourcePgUpdate,
		DeleteContext: resourcePgDelete,
		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
				Description: "The name of the Postgres service.",
			},
			"image": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				Description: "The Docker image to use for the Postgres service. If not specified, Dokku will use its default Postgres image.",
			},
			// TODO: locked support
			"image_version": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				Description: "The version of Postgres to use. If not specified, Dokku will use its default version.",
			},
			// We can't support these yet as there's no way to
			// retrieve them from dokku
			// "password": {
			// 	Type:     schema.TypeString,
			// 	Optional: true,
			// },
			// "root_password": {
			// 	Type:     schema.TypeString,
			// 	Optional: true,
			// },
			// "custom_env": {
			// 	Type:     schema.TypeString,
			// 	Optional: true,
			// },
			"stopped": {
				Type:     schema.TypeBool,
				Optional: true,
				Computed: true,
				Description: "Whether the Postgres service is stopped. When true, the database service will not be running but data will be preserved.",
			},
			"expose_on": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Network address and port to expose the service on. Format is 'host:port' (e.g. '0.0.0.0:8085'). If not specified, the service remains unexposed.",
				// TODO validator?
			},
			// TODO backup related stuff
			// "backup_auth_access_key": {
			// 	Type:     schema.TypeString,
			// 	Optional: true,
			// },
		},
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
	}
}

func resourcePgCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	sshClient := m.(*goph.Client)

	var diags diag.Diagnostics

	pg := NewDokkuPostgresServiceFromResourceData(d)
	err := dokkuPgCreate(pg, sshClient)

	if err != nil {
		return diag.FromErr(err)
	}

	pg.setOnResourceData(d)

	// TODO stop if necessary

	return diags
}

func resourcePgRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	sshClient := m.(*goph.Client)

	var diags diag.Diagnostics

	var serviceName string
	if d.Id() == "" {
		serviceName = d.Get("name").(string)
	} else {
		serviceName = d.Id()
	}

	pg := NewDokkuPostgresService(serviceName)
	err := dokkuPgRead(pg, sshClient)

	if err != nil {
		return diag.FromErr(err)
	}

	pg.setOnResourceData(d)

	return diags
}

func resourcePgUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	sshClient := m.(*goph.Client)

	var diags diag.Diagnostics

	pg := NewDokkuPostgresServiceFromResourceData(d)
	err := dokkuPgUpdate(pg, d, sshClient)

	if err != nil {
		return diag.FromErr(err)
	}

	pg.setOnResourceData(d)

	return diags
}

func resourcePgDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	sshClient := m.(*goph.Client)

	var diags diag.Diagnostics

	err := dokkuPgDestroy(NewDokkuPostgresService(d.Id()), sshClient)

	if err != nil {
		return diag.FromErr(err)
	}

	return diags
}
