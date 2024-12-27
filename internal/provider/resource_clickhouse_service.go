package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/melbahja/goph"
)

// Had issues with other images and cloning (not implemented at time of writing)
// with clickhouse. PR's welcome to implement this behaviour.
//
// This is therefore a less complete resource than e.g postgres, mysql

func resourceClickhouseService() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceChCreate,
		ReadContext:   resourceChRead,
		UpdateContext: resourceChUpdate,
		DeleteContext: resourceChDelete,
		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"stopped": {
				Type:     schema.TypeBool,
				Optional: true,
				Computed: true,
			},
			"exposed_on": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Network address and port to expose the service on. Format is 'host:port' (e.g. '0.0.0.0:8085'). If not specified, the service remains unexposed.",
				// TODO validator?
			},
		},
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
	}
}

func resourceChCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	sshClient := m.(*goph.Client)

	var diags diag.Diagnostics

	res := run(sshClient, fmt.Sprintf("clickhouse:create %s", d.Get("name").(string)))

	if res.err != nil {
		return diag.FromErr(res.err)
	}

	d.SetId(d.Get("name").(string))

	if d.Get("stopped").(bool) {
		res = run(sshClient, fmt.Sprintf("clickhouse:stop %s", d.Id()))

		if res.err != nil {
			return diag.FromErr(res.err)
		}
	}

	return diags
}

func resourceChRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	sshClient := m.(*goph.Client)

	var diags diag.Diagnostics

	serviceInfo, err := getServiceInfo("clickhouse", d.Id(), sshClient)

	if err != nil {
		return diag.FromErr(err)
	}

	if serviceInfo == nil {
		d.SetId("")
		return diags
	}

	if status, ok := serviceInfo["status"]; ok {
		d.Set("stopped", status == "exited" || status == "missing")
	}

	return diags
}

func resourceChUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	sshClient := m.(*goph.Client)

	var diags diag.Diagnostics

	if d.HasChange("stopped") {
		var res SshOutput

		isStopped := d.Get("stopped").(bool)
		if isStopped {
			res = run(sshClient, fmt.Sprintf("clickhouse:stop %s", d.Id()))
		} else {
			res = run(sshClient, fmt.Sprintf("clickhouse:start %s", d.Id()))
		}

		if res.err != nil {
			return diag.FromErr(res.err)
		}
	}

	return diags
}

func resourceChDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	sshClient := m.(*goph.Client)

	var diags diag.Diagnostics

	res := run(sshClient, fmt.Sprintf("clickhouse:destroy %s -f", d.Id()))

	if res.err != nil {
		return diag.FromErr(res.err)
	}

	d.SetId("")

	return diags
}
