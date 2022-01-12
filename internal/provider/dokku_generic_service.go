package provider

//
// Reusable logic for different services (pg/redis etc)
//

import (
	"fmt"
	"log"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/melbahja/goph"
)

type DokkuGenericServiceI interface {
	setOnResourceData(d *schema.ResourceData)
}

type DokkuGenericService struct {
	Id           string
	Name         string
	Image        string
	ImageVersion string
	// Password     string
	// RootPassword string
	// CustomEnv    string
	Stopped bool

	CmdName string
}

//
func (s *DokkuGenericService) setOnResourceData(d *schema.ResourceData) {
	d.SetId(s.Id)
	d.Set("name", s.Name)
	d.Set("image", s.Image)
	d.Set("image_version", s.ImageVersion)
	// d.Set("password", s.Password)
	// d.Set("root_password", s.RootPassword)
	// d.Set("custom_env", s.CustomEnv)
	d.Set("stopped", s.Stopped)
}

func (s *DokkuGenericService) Cmd(str ...string) string {
	return fmt.Sprintf("%s:%s", s.CmdName, strings.Join(str, " "))
}

//
func createServiceFlagStr(service *DokkuGenericService, flagsToAddSlice ...string) string {
	addAllFlags := len(flagsToAddSlice) == 0
	flagsToAdd := sliceToLookupMap(flagsToAddSlice)
	flags := make([]string, 1)

	if service.Image != "" {
		if _, ok := flagsToAdd["image"]; ok || addAllFlags {
			flags = append(flags, fmt.Sprintf("--image %s", service.Image))
		}
	}

	if service.ImageVersion != "" {
		if _, ok := flagsToAdd["image-version"]; ok || addAllFlags {
			flags = append(flags, fmt.Sprintf("--image-version %s", service.ImageVersion))
		}
	}

	// if service.Password != "" {
	// 	if _, ok := flagsToAdd["password"]; ok || addAllFlags {
	// 		flags = append(flags, fmt.Sprintf("--password %s", service.Password))
	// 	}
	// }

	// if service.RootPassword != "" {
	// 	if _, ok := flagsToAdd["root-password"]; ok || addAllFlags {
	// 		flags = append(flags, fmt.Sprintf("--root-password %s", service.RootPassword))
	// 	}
	// }

	// if service.CustomEnv != "" {
	// 	if _, ok := flagsToAdd["custom-env"]; ok || addAllFlags {
	// 		flags = append(flags, fmt.Sprintf("--custom-env %s", service.CustomEnv))
	// 	}
	// }

	return strings.Join(flags, " ")
}

//
func dokkuServiceRead(service *DokkuGenericService, client *goph.Client) error {
	serviceInfo, err := getServiceInfo(service.CmdName, service.Name, client)

	if err != nil {
		return err
	}

	if serviceInfo != nil {
		service.Id = service.Name
	}

	if status, ok := serviceInfo["status"]; ok {
		service.Stopped = status == "exited"
	}

	if version, ok := serviceInfo["version"]; ok {
		service.Image, service.ImageVersion = dockerImageAndVersion(version)
	}

	return nil
}

// When implementing clickhouse for v0.4.0 generic service didn't quite fit the
// bill, as we cannot yet support image etc.
//
// Probably the way we want to go in the future is just using a lower level API
// for extracting info from dokku. Adding this now allows us to re-use this
// in `dokkuServiceRead` as well as in the clickhouse service resource.
func getServiceInfo(service string, name string, client *goph.Client) (map[string]string, error) {
	res := run(client, fmt.Sprintf("%s:info %s", service, name))

	if res.err != nil {
		if res.status > 0 {
			log.Printf("[DEBUG] %s service %s does not exist\n", service, name)
			// return nil, err
			return nil, nil
		} else {
			return nil, res.err
		}
	}

	infoLines := strings.Split(res.stdout, "\n")[1:]

	data := make(map[string]string)

	for _, ln := range infoLines {
		lnPart := strings.Split(ln, ":")
		valPart := strings.TrimSpace(strings.Join(lnPart[1:], ":"))

		data[strings.TrimSpace(strings.ToLower(lnPart[0]))] = valPart
	}

	log.Printf("Returning info for %s service %s :: %v", service, name, data)

	return data, nil
}

//
func dokkuServiceCreate(service *DokkuGenericService, client *goph.Client) error {
	res := run(client, fmt.Sprintf("%s:create %s %s", service.CmdName, service.Name, createServiceFlagStr(service)))

	if res.err != nil {
		return res.err
	} else {
		// Service was created, stop it if necessary
		if service.Stopped {
			res = run(client, fmt.Sprintf("%s:stop %s", service.CmdName, service.Name))

			if res.err != nil {
				return res.err
			}
		}

		// Read the service to get info on image etc
		return dokkuServiceRead(service, client)
	}
}

//
func dokkuServiceUpdate(service *DokkuGenericService, d *schema.ResourceData, client *goph.Client) error {
	serviceName := d.Get("name").(string)
	oldServiceName := d.Get("name").(string)

	if d.HasChange("name") {
		oldServiceNameI, _ := d.GetChange("name")
		oldServiceName = oldServiceNameI.(string)
	}

	if d.HasChanges("name", "password", "root_password") {
		// Service needs to be recreated from scratch. We do this via `dokku service:clone`

		// If the name _wasn't_ changed, then we need to perform 2x clones, one
		// to a temporary db, delete the original, then clone again back to the original name
		var cloneServiceName string
		if !d.HasChange("name") {
			cloneServiceName = fmt.Sprintf("tf-tmp-%s-%s", serviceName, tmpResourceName(5))
		} else {
			cloneServiceName = serviceName
		}

		log.Printf("[DEBUG] running dokku %s:clone %s -> %s\n", service.CmdName, oldServiceName, cloneServiceName)
		createFlags := createServiceFlagStr(service)
		res := run(client, fmt.Sprintf("%s:clone %s %s %s\n", service.CmdName, oldServiceName, cloneServiceName, createFlags))

		if res.err != nil {
			return res.err
		}

		err := dokkuServiceDestroy(service.CmdName, oldServiceName, client)
		if err != nil {
			return err
		}

		if !d.HasChange("name") {
			// Clone again to the original name
			log.Printf("[DEBUG] running dokku %s:clone %s -> %s\n", service.CmdName, cloneServiceName, d.Get("name"))
			res = run(client, fmt.Sprintf("%s:clone %s %s %s\n", service.CmdName, cloneServiceName, d.Get("name"), createFlags))

			if res.err != nil {
				return res.err
			}

			err = dokkuServiceDestroy(service.CmdName, cloneServiceName, client)

			if err != nil {
				return err
			}
		}
	}

	service.Id = serviceName

	if d.HasChanges("image", "image_version", "custom_env") {
		log.Printf("[DEBUG] running %s:upgrade\n", serviceName)
		flags := createServiceFlagStr(service, "image", "image-version", "custom-env")
		updateStr := fmt.Sprintf("%s:upgrade %s %s", service.CmdName, service.Name, flags)

		log.Printf("[DEBUG] running `dokku %s`\n", updateStr)

		res := run(client, updateStr)

		if res.err != nil {
			return res.err
		}
	}

	if d.HasChange("stopped") {
		var res SshOutput
		if d.Get("stopped").(bool) {
			res = run(client, fmt.Sprintf("%s:stop %s", service.CmdName, service.Name))
		} else {
			res = run(client, fmt.Sprintf("%s:start %s", service.CmdName, service.Name))
		}

		if res.err != nil {
			return res.err
		}
	}

	return dokkuServiceRead(service, client)
}

//
func dokkuServiceDestroy(cmd string, serviceName string, client *goph.Client) error {
	log.Printf("[DEBUG] running %s:destroy on %s\n", cmd, serviceName)
	res := run(client, fmt.Sprintf("%s:destroy %s -f", cmd, serviceName))

	return res.err
}
