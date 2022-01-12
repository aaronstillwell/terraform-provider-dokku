package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/melbahja/goph"
)

func resourceRedisServiceLink() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceRedisServiceLinkCreate,
		ReadContext:   resourceRedisServiceLinkRead,
		DeleteContext: resourceRedisServiceLinkDelete,
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

const redisServiceCmd = "redis"

//
func resourceRedisServiceLinkCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	err := serviceLinkCreate(d, redisServiceCmd, m.(*goph.Client))

	if err != nil {
		return diag.FromErr(err)
	}

	return nil
}

//
func resourceRedisServiceLinkRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	err := serviceLinkRead(d, redisServiceCmd, m.(*goph.Client))

	if err != nil {
		return diag.FromErr(err)
	}

	return nil
}

//
func resourceRedisServiceLinkDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	err := serviceLinkDelete(d, redisServiceCmd, m.(*goph.Client))

	if err != nil {
		return diag.FromErr(err)
	}

	return nil
}
