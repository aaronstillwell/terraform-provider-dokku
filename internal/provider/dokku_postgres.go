package provider

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/melbahja/goph"
)

//
type DokkuPostgresService struct {
	DokkuGenericService
}

//
func NewDokkuPostgresService(name string) *DokkuPostgresService {
	return &DokkuPostgresService{
		DokkuGenericService: DokkuGenericService{
			Name:    name,
			CmdName: "postgres",
		},
	}
}

//
func NewDokkuPostgresServiceFromResourceData(d *schema.ResourceData) *DokkuPostgresService {
	return &DokkuPostgresService{
		DokkuGenericService: DokkuGenericService{
			Name:         d.Get("name").(string),
			Image:        d.Get("image").(string),
			ImageVersion: d.Get("image_version").(string),
			// Password:     d.Get("password").(string),
			// RootPassword: d.Get("root_password").(string),
			// CustomEnv:    d.Get("custom_env").(string),
			Stopped: d.Get("stopped").(bool),

			CmdName: "postgres",
		},
	}
}

func dokkuPgRead(pg *DokkuPostgresService, client *goph.Client) error {
	return dokkuServiceRead(&pg.DokkuGenericService, client)
}

func dokkuPgCreate(pg *DokkuPostgresService, client *goph.Client) error {
	return dokkuServiceCreate(&pg.DokkuGenericService, client)
}

func dokkuPgUpdate(pg *DokkuPostgresService, d *schema.ResourceData, client *goph.Client) error {
	return dokkuServiceUpdate(&pg.DokkuGenericService, d, client)
}

func dokkuPgDestroy(pg *DokkuPostgresService, client *goph.Client) error {
	return dokkuServiceDestroy(pg.CmdName, pg.Name, client)
}
