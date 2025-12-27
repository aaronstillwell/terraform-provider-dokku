package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/melbahja/goph"
)

func resourceMongodbService() *schema.Resource {
	return &schema.Resource{
		Description:   "Manages a MongoDB database service, allowing for creation and configuration of MongoDB databases. Requires the MongoDB Dokku plugin to be installed.",
		CreateContext: resourceMongodbCreate,
		ReadContext:   resourceMongodbRead,
		UpdateContext: resourceMongodbUpdate,
		DeleteContext: resourceMongodbDelete,
		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The name of the MongoDB service.",
			},
			"image": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "The Docker image to use for the MongoDB service. If not specified, Dokku will use its default MongoDB image.",
			},
			"image_version": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "The version of MongoDB to use. If not specified, Dokku will use its default version.",
			},
			"stopped": {
				Type:        schema.TypeBool,
				Optional:    true,
				Computed:    true,
				Description: "Whether the MongoDB service is stopped. When true, the database service will not be running but data will be preserved.",
			},
			"expose_on": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Network address and port to expose the service on. Format is 'host:port' (e.g. '0.0.0.0:8085'). If not specified, the service remains unexposed.",
			},
		},
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
	}
}

func resourceMongodbCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	sshClient := m.(*goph.Client)

	var diags diag.Diagnostics

	mongodb := NewDokkuMongodbServiceFromResourceData(d)
	err := dokkuMongodbCreate(mongodb, sshClient)

	if err != nil {
		return diag.FromErr(err)
	}

	mongodb.setOnResourceData(d)

	return diags
}

func resourceMongodbRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	sshClient := m.(*goph.Client)

	var diags diag.Diagnostics

	var serviceName string
	if d.Id() == "" {
		serviceName = d.Get("name").(string)
	} else {
		serviceName = d.Id()
	}

	mongodb := NewDokkuMongodbService(serviceName)
	err := dokkuMongodbRead(mongodb, sshClient)

	if err != nil {
		return diag.FromErr(err)
	}

	mongodb.setOnResourceData(d)

	return diags
}

func resourceMongodbUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	sshClient := m.(*goph.Client)

	var diags diag.Diagnostics

	mongodb := NewDokkuMongodbServiceFromResourceData(d)
	err := dokkuMongodbUpdate(mongodb, d, sshClient)

	if err != nil {
		return diag.FromErr(err)
	}

	mongodb.setOnResourceData(d)

	return diags
}

func resourceMongodbDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	sshClient := m.(*goph.Client)

	var diags diag.Diagnostics

	err := dokkuMongodbDestroy(NewDokkuMongodbService(d.Id()), sshClient)

	if err != nil {
		return diag.FromErr(err)
	}

	return diags
}
