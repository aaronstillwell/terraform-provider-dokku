package provider

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/melbahja/goph"
)

type DokkuMongodbService struct {
	DokkuGenericService
}

func NewDokkuMongodbService(name string) *DokkuMongodbService {
	return &DokkuMongodbService{
		DokkuGenericService: DokkuGenericService{
			Name:    name,
			CmdName: "mongodb",
		},
	}
}

func NewDokkuMongodbServiceFromResourceData(d *schema.ResourceData) *DokkuMongodbService {
	isStoppedI, isStoppedSet := d.GetOk("stopped")

	var isStopped bool
	if isStoppedSet {
		isStopped = isStoppedI.(bool)
	} else {
		isStopped = false
	}

	return &DokkuMongodbService{
		DokkuGenericService: DokkuGenericService{
			Name:         d.Get("name").(string),
			Image:        d.Get("image").(string),
			ImageVersion: d.Get("image_version").(string),
			Stopped:      isStopped,
			Exposed:      d.Get("expose_on").(string),

			CmdName: "mongodb",
		},
	}
}

func dokkuMongodbRead(mongodb *DokkuMongodbService, client *goph.Client) error {
	return dokkuServiceRead(&mongodb.DokkuGenericService, client)
}

func dokkuMongodbCreate(mongodb *DokkuMongodbService, client *goph.Client) error {
	return dokkuServiceCreate(&mongodb.DokkuGenericService, client)
}

func dokkuMongodbUpdate(mongodb *DokkuMongodbService, d *schema.ResourceData, client *goph.Client) error {
	return dokkuServiceUpdate(&mongodb.DokkuGenericService, d, client)
}

func dokkuMongodbDestroy(mongodb *DokkuMongodbService, client *goph.Client) error {
	return dokkuServiceDestroy(mongodb.CmdName, mongodb.Name, client)
}
