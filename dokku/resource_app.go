package dokku

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/melbahja/goph"
)

func resourceApp() *schema.Resource {
	return &schema.Resource{
		CreateContext: appCreate,
		ReadContext:   appRead,
		UpdateContext: appUpdate,
		DeleteContext: appDelete,
		Schema: map[string]*schema.Schema{
			"name": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			// TODO: locked support
			"locked": &schema.Schema{
				Type:     schema.TypeBool,
				Default:  false,
				Optional: true,
			},
			"config_vars": &schema.Schema{
				Type: schema.TypeMap,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Optional:  true,
				Sensitive: true,
			},
			"domains": &schema.Schema{
				Type: schema.TypeSet,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Optional: true,
			},
		},
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
	}
}

func appCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	sshClient := m.(*goph.Client)

	var diags diag.Diagnostics

	app := NewDokkuAppFromResourceData(d)

	dokkuAppCreate(app, sshClient)

	d.SetId(app.Name)

	return diags
}

//
func appRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	sshClient := m.(*goph.Client)

	var diags diag.Diagnostics

	var appName string
	if d.Id() != "" {
		appName = d.Id()
	} else {
		appName = d.Get("name").(string)
	}

	app, err := dokkuAppRetrieve(appName, sshClient)
	if err != nil {
		return diag.FromErr(err)
	}
	app.setOnResourceData(d)

	return diags
}

//
func appUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	app := NewDokkuAppFromResourceData(d)
	err := dokkuAppUpdate(app, d, m.(*goph.Client))

	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(d.Get("name").(string))

	return diags
}

//
func appDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	sshClient := m.(*goph.Client)

	var diags diag.Diagnostics

	appName := d.Get("name").(string)

	_, err := sshClient.Run(fmt.Sprintf("apps:destroy %s --force", appName))

	if err != nil {
		return diag.FromErr(err)
	}

	return diags
}
