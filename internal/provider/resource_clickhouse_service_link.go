package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/melbahja/goph"
)

func resourceClickhouseServiceLink() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceClickhouseServiceLinkCreate,
		ReadContext:   resourceClickhouseServiceLinkRead,
		DeleteContext: resourceClickhouseServiceLinkDelete,
		Schema: map[string]*schema.Schema{
			"service": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"app": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"alias": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"query_string": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
		},
	}
}

const clickhouseServiceCmd = "clickhouse"

//
func resourceClickhouseServiceLinkCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	err := serviceLinkCreate(d, clickhouseServiceCmd, m.(*goph.Client))

	var diags diag.Diagnostics

	if err != nil {
		return diag.FromErr(err)
	}

	return diags
}

//
func resourceClickhouseServiceLinkRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	err := serviceLinkRead(d, clickhouseServiceCmd, m.(*goph.Client))

	var diags diag.Diagnostics

	if err != nil {
		return diag.FromErr(err)
	}

	return diags
}

//
func resourceClickhouseServiceLinkDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	err := serviceLinkDelete(d, clickhouseServiceCmd, m.(*goph.Client))

	var diags diag.Diagnostics

	if err != nil {
		return diag.FromErr(err)
	}

	return diags
}
