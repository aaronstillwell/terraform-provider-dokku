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
	Exposed []string

	CmdName string
}

func (s *DokkuGenericService) setOnResourceData(d *schema.ResourceData) {
	d.SetId(s.Id)
	d.Set("name", s.Name)
	d.Set("image", s.Image)
	d.Set("image_version", s.ImageVersion)
	// d.Set("password", s.Password)
	// d.Set("root_password", s.RootPassword)
	// d.Set("custom_env", s.CustomEnv)
	d.Set("stopped", s.Stopped)
	d.Set("expose_on", s.buildExposedPortsString())
}

func (s *DokkuGenericService) Cmd(str ...string) string {
	return fmt.Sprintf("%s:%s", s.CmdName, strings.Join(str, " "))
}

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

func parseExposedPorts(exposedPorts string) ([]string, error) {
	if exposedPorts == "-" {
		return nil, nil
	}

	ports := strings.Split(exposedPorts, "->")
	if len(ports) <= 1 {
		return nil, fmt.Errorf("invalid port mapping format: %s", exposedPorts)
	}

	result := make([]string, 0)
	for i := 1; i < len(ports); i += 2 {
		result = append(result, strings.TrimSpace(ports[i]))
	}
	return result, nil
}

func dokkuServiceRead(service *DokkuGenericService, client *goph.Client) error {
	serviceInfo, err := getServiceInfo(service.CmdName, service.Name, client)

	if err != nil {
		return err
	}

	if serviceInfo != nil {
		service.Id = service.Name
	}

	if status, ok := serviceInfo["status"]; ok {
		service.Stopped = status == "exited" || status == "missing"
	}

	log.Printf("[DEBUG] %s status %s", service.Name, serviceInfo["status"])
	log.Printf("[DEBUG] %s stopped? %t", service.Name, service.Stopped)

	// cannot read image & version if the service is stopped
	if !service.Stopped {
		if version, ok := serviceInfo["version"]; ok {
			service.Image, service.ImageVersion = dockerImageAndVersion(version)
		}
	}

	if exposedPorts, ok := serviceInfo["exposed ports"]; ok {
		parsedPorts, err := parseExposedPorts(exposedPorts)
		if err != nil {
			return err
		}
		service.Exposed = parsedPorts
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

func (s *DokkuGenericService) buildExposedPortsString() string {
	return strings.Join(s.Exposed, " ")
}

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

		// Expose the service if necessary
		log.Printf("[DEBUG] expose_on set to %v", service.Exposed)
		// Note that we're checking here that service.Exposed isn't simply []string{""} - this is what
		// strings.Split(d.Get("expose_on").(string), " ") would return by default on an empty string
		if len(service.Exposed) > 0 && !(len(service.Exposed) == 1 && service.Exposed[0] == "") {
			log.Printf("[DEBUG] service.Exposed length %d", len(service.Exposed))
			log.Print("[DEBUG] Setting expose...")
			res = run(client, fmt.Sprintf("%s:expose %s %s", service.CmdName, service.Name, service.buildExposedPortsString()))
			if res.err != nil {
				return res.err
			}
		}

		// Read the service to get info on image etc
		return dokkuServiceRead(service, client)
	}
}

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

		// The clone keeps the image of the service it copies. Any change to the
		// image is applied afterwards by the upgrade below, because the 2.x
		// datastore plugins refuse to clone onto an image that runs on another
		// definition (e.g. another postgres major version).
		log.Printf("[DEBUG] running dokku %s:clone %s -> %s\n", service.CmdName, oldServiceName, cloneServiceName)
		res := run(client, fmt.Sprintf("%s:clone %s %s", service.CmdName, oldServiceName, cloneServiceName))

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
			res = run(client, fmt.Sprintf("%s:clone %s %s", service.CmdName, cloneServiceName, d.Get("name")))

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
		if err := dokkuServiceUpgrade(service, client); err != nil {
			return err
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

	if d.HasChange("expose_on") {
		var res SshOutput
		exposed := d.Get("expose_on").(string)
		if exposed == "" {
			res = run(client, fmt.Sprintf("%s:unexpose %s", service.CmdName, service.Name))
		} else {
			res = run(client, fmt.Sprintf("%s:expose %s %s", service.CmdName, service.Name, exposed))
		}

		if res.err != nil {
			return res.err
		}
	}

	return dokkuServiceRead(service, client)
}

// restartAppsRequiredMsg is part of the error the 2.x datastore plugins return
// when an upgrade moves a service onto another definition (e.g. another major
// version) and so has to migrate its data.
const restartAppsRequiredMsg = "needs --restart-apps"

// dokkuServiceUpgrade moves a service onto the configured image and version.
//
// An upgrade that stays on the same definition is run as-is, so linked apps are
// left running. With the 2.x plugins, an upgrade onto another definition
// migrates the data and is refused unless --restart-apps is passed, so it is
// retried with that flag; linked apps are stopped while the data is copied.
// The 1.x plugins never return this error, and their --restart-apps takes a
// value, so the flag is only ever added in response to it.
func dokkuServiceUpgrade(service *DokkuGenericService, client *goph.Client) error {
	flags := createServiceFlagStr(service, "image", "image-version", "custom-env")
	upgradeStr := fmt.Sprintf("%s:upgrade %s %s", service.CmdName, service.Name, flags)

	log.Printf("[DEBUG] running `dokku %s`\n", upgradeStr)
	res := run(client, upgradeStr)

	if res.err != nil && strings.Contains(res.stdout, restartAppsRequiredMsg) {
		upgradeStr = fmt.Sprintf("%s --restart-apps", upgradeStr)
		log.Printf("[DEBUG] upgrade migrates data, running `dokku %s`\n", upgradeStr)
		res = run(client, upgradeStr)
	}

	return res.err
}

func dokkuServiceDestroy(cmd string, serviceName string, client *goph.Client) error {
	log.Printf("[DEBUG] running %s:destroy on %s\n", cmd, serviceName)
	res := run(client, fmt.Sprintf("%s:destroy %s -f", cmd, serviceName))

	return res.err
}
