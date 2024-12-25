package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/melbahja/goph"
)

func resourcePostgresService() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourcePgCreate,
		ReadContext:   resourcePgRead,
		UpdateContext: resourcePgUpdate,
		DeleteContext: resourcePgDelete,
		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"image": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			// TODO: locked support
			"image_version": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
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
			},
			"exposed": {
				Type:     schema.TypeString,
				Optional: true,
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

//
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

//
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

//
func resourcePgDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	sshClient := m.(*goph.Client)

	var diags diag.Diagnostics

	err := dokkuPgDestroy(NewDokkuPostgresService(d.Id()), sshClient)

	if err != nil {
		return diag.FromErr(err)
	}

	return diags
}
