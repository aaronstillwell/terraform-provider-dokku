package provider

import (
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/melbahja/goph"
)

type DokkuMysqlService struct {
	DokkuGenericService
}

func NewMysqlService(name string) *DokkuMysqlService {
	return &DokkuMysqlService{
		DokkuGenericService: DokkuGenericService{
			Name:    name,
			CmdName: "mysql",
		},
	}
}

func NewMysqlServiceFromResourceData(d *schema.ResourceData) *DokkuMysqlService {
	return &DokkuMysqlService{
		DokkuGenericService: DokkuGenericService{
			Name:         d.Get("name").(string),
			Image:        d.Get("image").(string),
			ImageVersion: d.Get("image_version").(string),
			Stopped:      d.Get("stopped").(bool),
			Exposed:      strings.Split(d.Get("expose_on").(string), " "),
			CmdName:      "mysql",
		},
	}
}

func dokkuMysqlRead(mysql *DokkuMysqlService, client *goph.Client) error {
	return dokkuServiceRead(&mysql.DokkuGenericService, client)
}

func dokkuMysqlCreate(mysql *DokkuMysqlService, client *goph.Client) error {
	return dokkuServiceCreate(&mysql.DokkuGenericService, client)
}

func dokkuMysqlUpdate(mysql *DokkuMysqlService, d *schema.ResourceData, client *goph.Client) error {
	return dokkuServiceUpdate(&mysql.DokkuGenericService, d, client)
}

func dokkuMysqlDestroy(mysql *DokkuMysqlService, client *goph.Client) error {
	return dokkuServiceDestroy(mysql.CmdName, mysql.Name, client)
}
