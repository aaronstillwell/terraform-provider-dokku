package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/melbahja/goph"
)

func resourceRedisServiceLink() *schema.Resource {
	return &schema.Resource{
		Description: "Links a Dokku Redis service to an application, creating a connection between them and injecting the redis connection details into the application's environment variables.",
		CreateContext: resourceRedisServiceLinkCreate,
		ReadContext:   resourceRedisServiceLinkRead,
		DeleteContext: resourceRedisServiceLinkDelete,
		Schema: map[string]*schema.Schema{
			"service": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
				Description: "The name of the Redis service to link to the application.",
			},
			"app": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
				Description: "The name of the Dokku application that will be linked to the Redis service.",
			},
			"alias": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
				Description: "Alternative environment variable name to use in exposing credentials to the app.",
			},
			"query_string": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
				Description: "Additional connection parameters to append to the REDIS_URL environment variable as a query string.",
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
