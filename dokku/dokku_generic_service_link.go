package dokku

import (
	"fmt"
	"log"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/melbahja/goph"
)

//
func serviceLinkCreate(d *schema.ResourceData, serviceName string, client *goph.Client) error {
	options := make([]string, 2)

	if _, ok := d.GetOk("alias"); ok {
		alias := fmt.Sprintf("--alias %s", d.Get("alias").(string))
		options = append(options, alias)
	}

	if _, ok := d.GetOk("query_string"); ok {
		query_string := fmt.Sprintf("--querystring %s", d.Get("query_string").(string))
		options = append(options, query_string)
	}

	optionsCmd := " " + strings.Join(options, " ")

	cmd := fmt.Sprintf("%s:link %s %s %s", serviceName, d.Get("service"), d.Get("app"), optionsCmd)
	log.Printf("[DEBUG] running `%s`", cmd)
	res := run(client, cmd)

	// TODO better error handling, e.g app already created
	if res.err != nil {
		log.Printf("[DEBUG] Could not create service link\n")
		log.Printf("[DEBUG] %s\n", res.err)
	}

	d.SetId(fmt.Sprintf("%s-%s", d.Get("service").(string), d.Get("app").(string)))

	return res.err
}

// Reading a service link is currently limited by the info we can get from dokku. We can only
// assess if a given link exists, rather than actually check the query string & alias
//
// thought: maybe we can get the alias from the app config?
//
// as such this function for now just assesses whether or not the link exists
func serviceLinkRead(d *schema.ResourceData, serviceName string, client *goph.Client) error {
	cmd := fmt.Sprintf("%s:linked %s %s", serviceName, d.Get("service"), d.Get("app"))
	log.Println(fmt.Sprintf("[DEBUG] running `%s`", cmd))
	res := run(client, cmd)

	d.SetId(fmt.Sprintf("%s-%s", d.Get("service").(string), d.Get("app").(string)))

	if res.err != nil {
		// TODO use stdout as extra verification?
		if res.status == 1 || res.status == 20 {
			d.SetId("")
			return nil
		}
	}

	return res.err
}

//
func serviceLinkDelete(d *schema.ResourceData, serviceName string, client *goph.Client) error {
	res := run(client, fmt.Sprintf("%s:unlink %s %s", serviceName, d.Get("service"), d.Get("app")))

	if res.err == nil {
		d.SetId("")
	}
	return res.err
}
