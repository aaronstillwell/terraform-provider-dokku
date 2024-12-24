package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/melbahja/goph"
)

func resourcePostgresServiceLink() *schema.Resource {
	return &schema.Resource{
		Description: "Links a Dokku Postgres service to an application, creating a connection between them and injecting the database connection details into the application's environment variables.",
		CreateContext: resourcePostgresServiceLinkCreate,
		ReadContext:   resourcePostgresServiceLinkRead,
		DeleteContext: resourcePostgresServiceLinkDelete,
		Schema: map[string]*schema.Schema{
			"service": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
				Description: "The name of the Postgres service to link to the application.",
			},
			"app": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
				Description: "The name of the Dokku application that will be linked to the Postgres service.",
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
				Description: "Additional connection parameters to append to the DATABASE_URL environment variable as a query string.",
			},
		},
	}
}

const pgServiceCmd = "postgres"

//
func resourcePostgresServiceLinkCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	err := serviceLinkCreate(d, pgServiceCmd, m.(*goph.Client))

	var diags diag.Diagnostics

	if err != nil {
		return diag.FromErr(err)
	}

	return diags
}

//
func resourcePostgresServiceLinkRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	err := serviceLinkRead(d, pgServiceCmd, m.(*goph.Client))

	var diags diag.Diagnostics

	if err != nil {
		return diag.FromErr(err)
	}

	return diags
}

//
func resourcePostgresServiceLinkDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	err := serviceLinkDelete(d, pgServiceCmd, m.(*goph.Client))

	var diags diag.Diagnostics

	if err != nil {
		return diag.FromErr(err)
	}

	return diags
}
