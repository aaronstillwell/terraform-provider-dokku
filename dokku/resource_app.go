package dokku

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
		CreateContext: appCreate,
		ReadContext:   appRead,
		UpdateContext: appUpdate,
		DeleteContext: appDelete,
		Schema: map[string]*schema.Schema{
			"name": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			// TODO: locked support
			"locked": &schema.Schema{
				Type:     schema.TypeBool,
				Default:  false,
				Optional: true,
			},
			"config_vars": &schema.Schema{
				Type: schema.TypeMap,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Optional:  true,
				Sensitive: true,
			},
			"domains": &schema.Schema{
				Type: schema.TypeSet,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Optional: true,
			},
			"buildpacks": &schema.Schema{
				Type: schema.TypeList,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Optional: true,
			},
			"ports": &schema.Schema{
				Type: schema.TypeSet,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Optional: true,
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
				Computed:     true,
				ValidateFunc: validation.IsIPv4Address,
			},
			"nginx_bind_address_ipv6": {
				Type:         schema.TypeString,
				Optional:     true,
				Default:      "::",
				ValidateFunc: validation.IsIPv6Address,
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

	d.SetId(app.Name)

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
