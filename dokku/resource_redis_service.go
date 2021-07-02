package dokku

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/melbahja/goph"
)

func resourceRedisService() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceRedisCreate,
		ReadContext:   resourceRedisRead,
		UpdateContext: resourceRedisUpdate,
		DeleteContext: resourceRedisDestroy,
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

//
func resourceRedisCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	sshClient := m.(*goph.Client)

	var diags diag.Diagnostics

	redis := NewDokkuRedisServiceFromResourceData(d)
	err := dokkuRedisCreate(redis, sshClient)

	if err != nil {
		return diag.FromErr(err)
	}

	redis.setOnResourceData(d)

	return diags
}

//
func resourceRedisRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	sshClient := m.(*goph.Client)

	var diags diag.Diagnostics

	var serviceName string
	if d.Id() == "" {
		serviceName = d.Get("name").(string)
	} else {
		serviceName = d.Id()
	}

	redis := NewDokkuRedisService(serviceName)
	err := dokkuRedisRead(redis, sshClient)

	if err != nil {
		return diag.FromErr(err)
	}

	redis.setOnResourceData(d)

	return diags
}

//
func resourceRedisUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	sshClient := m.(*goph.Client)

	var diags diag.Diagnostics

	redis := NewDokkuRedisServiceFromResourceData(d)
	err := dokkuRedisUpdate(redis, d, sshClient)

	if err != nil {
		return diag.FromErr(err)
	}

	return diags
}

//
func resourceRedisDestroy(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	sshClient := m.(*goph.Client)

	var diags diag.Diagnostics

	err := dokkuRedisDestroy(NewDokkuRedisService(d.Id()), sshClient)

	if err != nil {
		return diag.FromErr(err)
	}

	return diags
}
