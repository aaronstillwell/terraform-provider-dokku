package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/melbahja/goph"
)

func resourceApp() *schema.Resource {
	return &schema.Resource{
		Description: "Manages a Dokku application. This resource enables the configuration and deployment of applications on a Dokku host, supporting environment variables, domains, buildpacks, and port mapping.",
		CreateContext: appCreate,
		ReadContext:   appRead,
		UpdateContext: appUpdate,
		DeleteContext: appDelete,
		Schema: map[string]*schema.Schema{
			"name": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				Description: "The name of the Dokku application.",
			},
			// TODO: locked support
			"locked": &schema.Schema{
				Type:     schema.TypeBool,
				Default:  false,
				Optional: true,
				Description: "(Not yet implemented) Whether the application is locked for deployment. When true, deploys to this application will be blocked.",
			},
			"config_vars": &schema.Schema{
				Type: schema.TypeMap,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Optional:  true,
				Sensitive: true,
				Description: "Environment variables to set for the application. These are exposed to the application at runtime.",
			},
			"domains": &schema.Schema{
				Type: schema.TypeSet,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Optional: true,
				Computed: true,
				Description: "List of domains to be associated with the application.",
			},
			"buildpacks": &schema.Schema{
				Type: schema.TypeList,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Optional: true,
				Description: "List of buildpacks to be used when deploying the application. These can be URLs to custom buildpacks or shorthand names for official Heroku buildpacks.",
			},
			"ports": &schema.Schema{
				Type: schema.TypeSet,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Optional: true,
				Description: "Set of port mappings for the application. Each mapping should be in the format 'scheme:hostPort:containerPort' (e.g., 'https:443:8080').",
				// ValidateFunc: func(val interface{}, key string) (warns []string, errs []error) {
				// 	v := val.([]string)

				// 	for _, port := range v {
				// 		isValidPort, _ := regexp.MatchString(`[A-z]+:[0-9]+:[0-9]+`, port)
				// 		if !isValidPort {
				// 			errs = append(errs, fmt.Errorf("Invalid port, expected format scheme:hostPort:containerPort e.g https:443:8080"))
				// 		}
				// 	}
				// 	return []string{}, errs
				// },
			},
			"nginx_bind_address_ipv4": {
				Type:         schema.TypeString,
				Optional:     true,
				Default:      "0.0.0.0",
				ValidateFunc: validation.IsIPv4Address,
				Description: "The IPv4 address that nginx will bind to for this application. Defaults to '0.0.0.0'.",
			},
			"nginx_bind_address_ipv6": {
				Type:         schema.TypeString,
				Optional:     true,
				Default:      "::",
				ValidateFunc: validation.IsIPv6Address,
				Description: "The IPv6 address that nginx will bind to for this application. Defaults to '::'.",
			},
		},
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
	}
}

func appCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	sshClient := m.(*goph.Client)

	var diags diag.Diagnostics

	app := NewDokkuAppFromResourceData(d)

	err := dokkuAppCreate(app, sshClient)

	if err != nil {
		return diag.FromErr(err)
	}

	app, err = dokkuAppRetrieve(app.Name, sshClient)
	app.setOnResourceData(d)

	return diags
}

//
func appRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	sshClient := m.(*goph.Client)

	var diags diag.Diagnostics

	var appName string
	if d.Id() != "" {
		appName = d.Id()
	} else {
		appName = d.Get("name").(string)
	}

	app, err := dokkuAppRetrieve(appName, sshClient)
	if err != nil {
		return diag.FromErr(err)
	}
	app.setOnResourceData(d)

	return diags
}

//
func appUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	app := NewDokkuAppFromResourceData(d)
	err := dokkuAppUpdate(app, d, m.(*goph.Client))

	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(d.Get("name").(string))

	return diags
}

//
func appDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	sshClient := m.(*goph.Client)

	var diags diag.Diagnostics

	appName := d.Get("name").(string)

	res := run(sshClient, fmt.Sprintf("apps:destroy %s --force", appName))

	if res.err != nil {
		return diag.FromErr(res.err)
	}

	return diags
}
