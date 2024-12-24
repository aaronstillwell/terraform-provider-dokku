package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/melbahja/goph"
)

func resourceMysqlServiceLink() *schema.Resource {
	return &schema.Resource{
		Description: "Links a Dokku MySQL service to an application, creating a connection between them and injecting the MySQL connection details into the application's environment variables.",
		CreateContext: resourceMysqlServiceLinkCreate,
		ReadContext:   resourceMysqlServiceLinkRead,
		DeleteContext: resourceMysqlServiceLinkDelete,
		Schema: map[string]*schema.Schema{
			"service": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
				Description: "The name of the MySQL service to link to the application.",
			},
			"app": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
				Description: "The name of the Dokku application that will be linked to the MySQL service.",
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

const mysqlServiceCmd = "mysql"

//
func resourceMysqlServiceLinkCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	err := serviceLinkCreate(d, mysqlServiceCmd, m.(*goph.Client))

	var diags diag.Diagnostics

	if err != nil {
		return diag.FromErr(err)
	}

	return diags
}

//
func resourceMysqlServiceLinkRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	err := serviceLinkRead(d, mysqlServiceCmd, m.(*goph.Client))

	var diags diag.Diagnostics

	if err != nil {
		return diag.FromErr(err)
	}

	return diags
}

//
func resourceMysqlServiceLinkDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	err := serviceLinkDelete(d, mysqlServiceCmd, m.(*goph.Client))

	var diags diag.Diagnostics

	if err != nil {
		return diag.FromErr(err)
	}

	return diags
}
