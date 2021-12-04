package dokku

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/melbahja/goph"
)

func resourceMysqlService() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceMysqlCreate,
		ReadContext:   resourceMysqlRead,
		UpdateContext: resourceMysqlUpdate,
		DeleteContext: resourceMysqlDelete,
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
			"image_version": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"stopped": {
				Type:     schema.TypeBool,
				Optional: true,
				Computed: true,
			},
		},
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
	}
}

func resourceMysqlCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	sshClient := m.(*goph.Client)

	var diags diag.Diagnostics

	mysql := NewMysqlServiceFromResourceData(d)
	err := dokkuMysqlCreate(mysql, sshClient)

	if err != nil {
		return diag.FromErr(err)
	}

	mysql.setOnResourceData(d)

	return diags
}

func resourceMysqlRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	sshClient := m.(*goph.Client)

	var diags diag.Diagnostics

	var serviceName string
	if d.Id() == "" {
		serviceName = d.Get("name").(string)
	} else {
		serviceName = d.Id()
	}

	mysql := NewMysqlService(serviceName)
	err := dokkuMysqlRead(mysql, sshClient)

	if err != nil {
		return diag.FromErr(err)
	}

	mysql.setOnResourceData(d)

	return diags
}

func resourceMysqlUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	sshClient := m.(*goph.Client)

	var diags diag.Diagnostics

	mysql := NewMysqlServiceFromResourceData(d)
	err := dokkuMysqlUpdate(mysql, d, sshClient)

	if err != nil {
		return diag.FromErr(err)
	}

	mysql.setOnResourceData(d)

	return diags
}

func resourceMysqlDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	sshClient := m.(*goph.Client)

	var diags diag.Diagnostics

	err := dokkuMysqlDestroy(NewMysqlService(d.Id()), sshClient)

	if err != nil {
		return diag.FromErr(err)
	}

	return diags
}
