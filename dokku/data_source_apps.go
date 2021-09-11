package dokku

import (
	"context"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	"github.com/melbahja/goph"
)

func dataSourceApps() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceAppsRead,
		Schema: map[string]*schema.Schema{
			"apps": &schema.Schema{
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": &schema.Schema{
							Type:     schema.TypeString,
							Computed: true,
						},
					},
				},
			},
		},
	}
}

//
func dataSourceAppsRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*goph.Client)
	res := run(client, "apps:list")

	if res.err != nil {
		diag.FromErr(res.err)
	}

	appsOutput := res.stdout

	var diags diag.Diagnostics

	appOutputLines := strings.Split(appsOutput, "\n")

	log.Printf("[DEBUG] %v\n", appOutputLines)

	if strings.TrimSpace(appOutputLines[0]) != "=====> My Apps" {
		return diag.Errorf("dokku CLI output for `dokku apps:list` not as expected")
	}

	appNames := appOutputLines[1:]

	apps := make([]map[string]string, 1, 1)

	for _, appName := range appNames {
		if len(appName) == 0 {
			continue
		}
		log.Printf("[DEBUG] Found app %v\n", appName)
		apps = append(apps, map[string]string{
			"name": appName,
		})
	}

	d.Set("apps", apps)
	d.SetId(strconv.FormatInt(time.Now().Unix(), 10))

	return diags
}
