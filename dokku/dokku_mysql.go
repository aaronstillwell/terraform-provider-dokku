package dokku

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/melbahja/goph"
)

//
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
			CmdName:      "mysql",
		},
	}
}

func dokkuMysqlRead(ch *DokkuMysqlService, client *goph.Client) error {
	return dokkuServiceRead(&ch.DokkuGenericService, client)
}

func dokkuMysqlCreate(ch *DokkuMysqlService, client *goph.Client) error {
	return dokkuServiceCreate(&ch.DokkuGenericService, client)
}

func dokkuMysqlUpdate(ch *DokkuMysqlService, d *schema.ResourceData, client *goph.Client) error {
	return dokkuServiceUpdate(&ch.DokkuGenericService, d, client)
}

func dokkuMysqlDestroy(ch *DokkuMysqlService, client *goph.Client) error {
	return dokkuServiceDestroy(ch.CmdName, ch.Name, client)
}
