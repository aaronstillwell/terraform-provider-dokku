package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/melbahja/goph"
)

func resourceClickhouseServiceLink() *schema.Resource {
	return &schema.Resource{
		Description: "Links a Dokku ClickHouse service to an application, creating a connection between them and injecting the ClickHouse connection details into the application's environment variables.",
		CreateContext: resourceClickhouseServiceLinkCreate,
		ReadContext:   resourceClickhouseServiceLinkRead,
		DeleteContext: resourceClickhouseServiceLinkDelete,
		Schema: map[string]*schema.Schema{
			"service": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
				Description: "The name of the ClickHouse service to link to the application.",
			},
			"app": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
				Description: "The name of the Dokku application that will be linked to the ClickHouse service.",
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
				Description: "Additional connection parameters to append to the service URL environment variables as a query string.",
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
