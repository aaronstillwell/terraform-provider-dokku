package dokku

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/melbahja/goph"
)

//
type DokkuRedisService struct {
	DokkuGenericService
}

//
func NewDokkuRedisService(name string) *DokkuRedisService {
	return &DokkuRedisService{
		DokkuGenericService: DokkuGenericService{
			Name:    name,
			CmdName: "redis",
		},
	}
}

//
func NewDokkuRedisServiceFromResourceData(d *schema.ResourceData) *DokkuRedisService {
	isStoppedI, isStoppedSet := d.GetOk("stopped")

	var isStopped bool
	if isStoppedSet {
		isStopped = isStoppedI.(bool)
	} else {
		isStopped = false
	}

	return &DokkuRedisService{
		DokkuGenericService: DokkuGenericService{
			Name:         d.Get("name").(string),
			Image:        d.Get("image").(string),
			ImageVersion: d.Get("image_version").(string),
			Stopped:      isStopped,

			CmdName: "redis",
		},
	}
}

//
func dokkuRedisRead(redis *DokkuRedisService, client *goph.Client) error {
	return dokkuServiceRead(&redis.DokkuGenericService, client)
}

//
func dokkuRedisCreate(redis *DokkuRedisService, client *goph.Client) error {
	return dokkuServiceCreate(&redis.DokkuGenericService, client)
}

//
func dokkuRedisUpdate(redis *DokkuRedisService, d *schema.ResourceData, client *goph.Client) error {
	return dokkuServiceUpdate(&redis.DokkuGenericService, d, client)
}

//
func dokkuRedisDestroy(redis *DokkuRedisService, client *goph.Client) error {
	return dokkuServiceDestroy(redis.CmdName, redis.Name, client)
}
