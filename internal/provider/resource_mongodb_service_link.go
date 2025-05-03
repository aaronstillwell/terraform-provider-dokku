package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/melbahja/goph"
)

func resourceMongodbServiceLink() *schema.Resource {
	return &schema.Resource{
		Description:   "Links a Dokku MongoDB service to an application, creating a connection between them and injecting the MongoDB connection details into the application's environment variables.",
		CreateContext: resourceMongodbServiceLinkCreate,
		ReadContext:   resourceMongodbServiceLinkRead,
		DeleteContext: resourceMongodbServiceLinkDelete,
		Schema: map[string]*schema.Schema{
			"service": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The name of the MongoDB service to link to the application.",
			},
			"app": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The name of the Dokku application that will be linked to the MongoDB service.",
			},
			"alias": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Description: "Alternative environment variable name to use in exposing credentials to the app.",
			},
			"query_string": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Description: "Additional connection parameters to append to the MONGODB_URL environment variable as a query string.",
			},
		},
	}
}

const mongodbServiceCmd = "mongo"

func resourceMongodbServiceLinkCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	err := serviceLinkCreate(d, mongodbServiceCmd, m.(*goph.Client))

	var diags diag.Diagnostics

	if err != nil {
		return diag.FromErr(err)
	}

	return diags
}

func resourceMongodbServiceLinkRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	err := serviceLinkRead(d, mongodbServiceCmd, m.(*goph.Client))

	var diags diag.Diagnostics

	if err != nil {
		return diag.FromErr(err)
	}

	return diags
}

func resourceMongodbServiceLinkDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	err := serviceLinkDelete(d, mongodbServiceCmd, m.(*goph.Client))

	var diags diag.Diagnostics

	if err != nil {
		return diag.FromErr(err)
	}

	return diags
}
