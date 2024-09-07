package dokku

import (
	"fmt"
	"log"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/melbahja/goph"
)

//
type DokkuApp struct {
	Id         string
	Name       string
	Locked     bool
	ConfigVars map[string]string
	Domains    []string
	Buildpacks []string
	// slice of strings denoting schema:hostPort:containerPort
	Ports                []string
	NginxBindAddressIpv4 string
	NginxBindAddressIpv6 string
}

//
func (app *DokkuApp) setOnResourceData(d *schema.ResourceData) {
	d.SetId(app.Id)
	d.Set("name", app.Name)
	d.Set("locked", app.Locked)

	d.Set("config_vars", app.managedConfigVars(d))

	d.Set("domains", app.Domains)

	if d.HasChange("buildpacks") || len(app.Buildpacks) > 0 {
		d.Set("buildpacks", app.Buildpacks)
	}

	managedPorts := app.managedPorts(d)
	if len(managedPorts) > 0 {
		d.Set("ports", managedPorts)
	} else {
		d.Set("ports", nil)
	}

	d.Set("nginx_bind_address_ipv4", app.NginxBindAddressIpv4)
	d.Set("nginx_bind_address_ipv6", app.NginxBindAddressIpv6)
}

// Leave alone config vars that are set outside of terraform. This is one way
// to avoid vars that are set by dokku etc (e.g DOKKU_PROXY_PORT).
func (app *DokkuApp) managedConfigVars(d *schema.ResourceData) map[string]string {
	tfConfigKeyLookup := make(map[string]struct{})
	tfConfigVars := make(map[string]string)

	// Extract the keys that exist in d
	if c, ok := d.GetOk("config_vars"); ok {
		m := c.(map[string]interface{})
		for k := range m {
			tfConfigKeyLookup[k] = struct{}{}
		}
	}

	for varKey, varVal := range app.ConfigVars {
		if _, ok := tfConfigKeyLookup[varKey]; ok {
			tfConfigVars[varKey] = varVal
		}
	}

	return tfConfigVars
}

// Similar behaviour implemented for ports - there will be some managed outside
// of terraform that we do not want to remove
func (app *DokkuApp) managedPorts(d *schema.ResourceData) []string {
	tfPortsLookup := make(map[string]struct{})
	tfPorts := []string{}

	if c, ok := d.GetOk("ports"); ok {
		ports := c.(*schema.Set)
		for _, p := range interfaceSliceToStrSlice(ports.List()) {
			tfPortsLookup[p] = struct{}{}
		}
	}

	for _, appPort := range app.Ports {
		if _, ok := tfPortsLookup[appPort]; ok {
			tfPorts = append(tfPorts, appPort)
		}
	}

	return tfPorts
}

// TODO escape quotes
func (app *DokkuApp) configVarsStr() string {
	str := ""
	for k, v := range app.ConfigVars {
		if len(str) > 0 {
			str = str + " "
		}
		str = str + k + "=\"" + v + "\""
	}
	return str
}

func NewDokkuAppFromResourceData(d *schema.ResourceData) *DokkuApp {
	domains := interfaceSliceToStrSlice(d.Get("domains").(*schema.Set).List())
	buildpacks := interfaceSliceToStrSlice(d.Get("buildpacks").([]interface{}))
	ports := interfaceSliceToStrSlice(d.Get("ports").(*schema.Set).List())

	configVars := make(map[string]string)
	for ck, cv := range d.Get("config_vars").(map[string]interface{}) {
		configVars[ck] = cv.(string)
	}

	return &DokkuApp{
		Name:                 d.Get("name").(string),
		Locked:               d.Get("locked").(bool),
		ConfigVars:           configVars,
		Domains:              domains,
		Buildpacks:           buildpacks,
		Ports:                ports,
		NginxBindAddressIpv4: d.Get("nginx_bind_address_ipv4").(string),
		NginxBindAddressIpv6: d.Get("nginx_bind_address_ipv6").(string),
	}
}

//
func dokkuAppRetrieve(appName string, client *goph.Client) (*DokkuApp, error) {
	res := run(client, fmt.Sprintf("apps:exists %s", appName))

	app := &DokkuApp{Id: appName, Name: appName, Locked: false}

	if res.err != nil {
		if res.status > 0 {
			// App does not exist
			app.Id = ""
			log.Printf("[DEBUG] app %s does not exist\n", appName)
			// return nil, err
			return app, nil
		} else {
			return nil, res.err
		}
	}

	app.ConfigVars = readAppConfig(appName, client)
	domains, err := readAppDomains(appName, client)
	if err != nil {
		return nil, err
	}
	app.Domains = domains

	buildpacks, err := readAppBuildpacks(appName, client)
	if err != nil {
		return nil, err
	}
	app.Buildpacks = buildpacks

	// ssl, err := readAppSsl(appName, client)
	// if err != nil {
	// 	return nil, err
	// }
	// app.Ssl = ssl

	ports, err := readAppPorts(appName, client)
	if err != nil {
		return nil, err
	}
	app.Ports = ports

	nginxReport, err := readAppNginxReport(appName, client)
	if err != nil {
		return nil, err
	}
	app.NginxBindAddressIpv4 = nginxReport.BindAddressIpv4
	app.NginxBindAddressIpv6 = nginxReport.BindAddressIpv6

	return app, nil
}

// TODO error handling
func readAppConfig(appName string, sshClient *goph.Client) map[string]string {
	res := run(sshClient, fmt.Sprintf("config:show %s", appName))

	// if err {
	// 	// TODO
	// }

	configLines := strings.Split(res.stdout, "\n")

	// TODO validate first line of output

	keyPairs := configLines[1:]

	config := make(map[string]string)

	for _, kp := range keyPairs {
		kp = strings.TrimSpace(kp)
		if len(kp) > 0 {
			parts := strings.Split(kp, ":")
			configKey := strings.TrimSpace(parts[0])

			configVal := parts[1]
			if len(parts[1]) > 1 {
				configVal = strings.Join(parts[1:], ":")
			}
			configVal = strings.TrimSpace(configVal)

			config[configKey] = configVal
		}
	}

	return config
}

//
func readAppDomains(appName string, client *goph.Client) ([]string, error) {
	res := run(client, fmt.Sprintf("domains:report %s", appName))

	if res.err != nil {
		return nil, res.err
	}

	domainLines := strings.Split(res.stdout, "\n")[1:]

	for _, line := range domainLines {
		parts := strings.Split(line, ":")

		key := strings.TrimSpace(parts[0])

		if key == "Domains app vhosts" {
			domainList := strings.TrimSpace(parts[1])
			if domainList == "" {
				return []string{}, nil
			} else {
				return strings.Split(domainList, " "), nil
			}
		}
	}

	// TODO proper error handling
	return nil, nil
}

// TODO Some parsing logic here that is replicated elsewhere (e.g readAppDomains above)
// which we can make reusable
func readAppBuildpacks(appName string, client *goph.Client) ([]string, error) {
	res := run(client, fmt.Sprintf("buildpacks:list %s", appName))

	if res.err != nil {
		return nil, res.err
	}

	buildpackLines := strings.Split(res.stdout, "\n")[1:]
	buildpacks := []string{}

	for _, line := range buildpackLines {
		line = strings.TrimSpace(line)
		if len(line) > 0 {
			buildpacks = append(buildpacks, line)
		}
	}

	return buildpacks, nil
}

func readAppPorts(appName string, client *goph.Client) ([]string, error) {
	res := run(client, fmt.Sprintf("%s %s", portReadCmd(), appName))

	portsLines := strings.Split(res.stdout, "\n")

	// returns status code 1 if no ports set
	if len(portsLines) <= 2 || res.status > 0 {
		return []string{}, nil
	}
	portsLines = portsLines[2:]

	if res.err != nil {
		return nil, res.err
	}

	var portMapping []string

	for _, line := range portsLines {
		split := strings.Split(line, " ")
		var parts []string

		for _, str := range split {
			if strings.TrimSpace(str) != "" {
				parts = append(parts, str)
			}
		}

		if len(parts) == 3 {
			portMapping = append(portMapping, strings.Join(parts, ":"))
		}
	}

	return portMapping, nil
}

type DokkuAppNginxReport struct {
	BindAddressIpv4 string
	BindAddressIpv6 string
}

//
func readAppNginxReport(appName string, client *goph.Client) (DokkuAppNginxReport, error) {
	res := run(client, fmt.Sprintf("nginx:report %s", appName))

	report := DokkuAppNginxReport{}

	if res.err != nil {
		return report, res.err
	}

	stdoutLines := strings.Split(res.stdout, "\n")[1:]

	nginxOpts := parseKeyValues(stdoutLines)

	// Dokku uses 0.0.0.0 and :: for ipv4/ipv6 bind addresses respectively by
	// default. However, in the stdout ipv4 is shown as a blank string
	// as of writing (dokku v0.25.7). We therefore make our own assumptions here
	// if these properties contain blanks.

	if ipv4Addr, ok := nginxOpts["Nginx bind address ipv4"]; ok {
		report.BindAddressIpv4 = ipv4Addr
	} else {
		report.BindAddressIpv4 = "0.0.0.0"
	}

	if ipv6Addr, ok := nginxOpts["Nginx bind address ipv6"]; ok {
		report.BindAddressIpv6 = ipv6Addr
	} else {
		report.BindAddressIpv6 = "::"
	}

	return report, nil
}

//
func dokkuAppCreate(app *DokkuApp, client *goph.Client) error {
	res := run(client, fmt.Sprintf("apps:create %s", app.Name))

	log.Printf("[DEBUG] apps:create %v\n", res.stdout)

	if res.err != nil {
		return res.err
	}

	err := dokkuAppConfigVarsSet(app, client)

	if err != nil {
		return err
	}

	err = dokkuAppDomainsAdd(app, client)

	if err != nil {
		return err
	}

	err = dokkuAppBuildpackAdd(app.Name, app.Buildpacks, client)

	if err != nil {
		return err
	}

	err = dokkuAppPortsAdd(app.Name, app.Ports, client)

	if err != nil {
		return err
	}

	err = dokkuAppNginxOptSet(app.Name, "bind-address-ipv4", app.NginxBindAddressIpv4, client)

	if err != nil {
		return err
	}

	err = dokkuAppNginxOptSet(app.Name, "bind-address-ipv6", app.NginxBindAddressIpv6, client)

	return err
}

//
func dokkuAppConfigVarsSet(app *DokkuApp, client *goph.Client) error {
	configVarStr := app.configVarsStr()
	if len(configVarStr) == 0 {
		return nil
	}

	secrets := make([]string, 0, len(app.ConfigVars))
	for _, v := range app.ConfigVars {
		secrets = append(secrets, v)
	}

	res := run(client, fmt.Sprintf("config:set %s %s", app.Name, configVarStr), secrets...)
	return res.err
}

//
func dokkuAppConfigVarsUnset(app *DokkuApp, varsToUnset []string, client *goph.Client) error {
	if len(varsToUnset) == 0 {
		return nil
	}
	log.Printf("[DEBUG] Unsetting keys %v\n", varsToUnset)
	cmd := fmt.Sprintf("config:unset %s %s", app.Name, strings.Join(varsToUnset, " "))
	log.Printf("[DEBUG] running %s", cmd)
	res := run(client, cmd)

	return res.err
}

//
func dokkuAppDomainsAdd(app *DokkuApp, client *goph.Client) error {
	domainStr := strings.Join(app.Domains, " ")

	if len(domainStr) > 0 {
		res := run(client, fmt.Sprintf("domains:set %s %s", app.Name, domainStr))
		return res.err
	}
	return nil
}

// Add buildpacks to an app based on the DokkuApp instance
func dokkuAppBuildpackAdd(appName string, buildpacks []string, client *goph.Client) error {
	for _, pack := range buildpacks {
		pack = strings.TrimSpace(pack)
		if len(pack) > 0 {
			res := run(client, fmt.Sprintf("buildpacks:add %s %s", appName, pack))

			if res.err != nil {
				return res.err
			}
		}
	}
	return nil
}

//
func dokkuAppPortsAdd(appName string, ports []string, client *goph.Client) error {
	for _, portRange := range ports {
		portRange = strings.TrimSpace(portRange)
		if len(portRange) > 0 {
			res := run(client, fmt.Sprintf("%s %s %s", portAddCmd(), appName, portRange))

			if res.err != nil {
				return res.err
			}
		}
	}

	return nil
}

func dokkuAppNginxOptSet(appName string, property string, value string, client *goph.Client) error {
	res := run(client, fmt.Sprintf("nginx:set %s %s %s", appName, property, value))
	return res.err
}

//
func dokkuAppUpdate(app *DokkuApp, d *schema.ResourceData, client *goph.Client) error {
	if d.HasChange("name") {
		old, _ := d.GetChange("name")
		res := run(client, fmt.Sprintf("apps:rename %s %s", old.(string), d.Get("name")))
		log.Printf("[DEBUG] apps:rename %s %s : %v\n", old.(string), d.Get("name"), res.stdout)
		if res.err != nil {
			return res.err
		}
	}

	appName := d.Get("name").(string)

	if d.HasChange("config_vars") {
		log.Println("[DEBUG] Changing config keys...")

		oldConfigVarsI, newConfigVarsI := d.GetChange("config_vars")
		oldConfigVars := mapOfInterfacesToMapOfStrings(oldConfigVarsI.(map[string]interface{}))
		newConfigVar := mapOfInterfacesToMapOfStrings(newConfigVarsI.(map[string]interface{}))

		keysToDelete := calculateMissingKeys(newConfigVar, oldConfigVars)

		dokkuAppConfigVarsUnset(app, keysToDelete, client)

		// TODO shouldn't need to duplicate below we already have config set function
		// This is basically an upsert, and will update values even if they haven't changed

		keysToUpsert := make([]string, 0)
		upsertParts := make([]string, 0)
		secrets := make([]string, 0)
		for newK, newV := range newConfigVar {
			keysToUpsert = append(keysToUpsert, newK)
			upsertParts = append(upsertParts, fmt.Sprintf("%s=\"%s\"", newK, newV))
			secrets = append(secrets, newV)
		}

		if len(upsertParts) > 0 {
			log.Printf("[DEBUG] Setting keys %v\n", keysToUpsert)
			res := run(client, fmt.Sprintf("config:set %s %s", appName, strings.Join(upsertParts, " ")), secrets...)

			if res.err != nil {
				return res.err
			}
		}
	}

	if d.HasChange("domains") {
		oldDomainsI, newDomainsI := d.GetChange("domains")
		oldDomains := interfaceSliceToStrSlice(oldDomainsI.(*schema.Set).List())
		newDomains := interfaceSliceToStrSlice(newDomainsI.(*schema.Set).List())
		domainsToRemove := calculateMissingStrings(newDomains, oldDomains)

		// Remove domains
		oldDomainsStr := strings.Join(domainsToRemove, " ")

		if len(oldDomainsStr) > 0 {
			res := run(client, fmt.Sprintf("domains:remove %s %s", appName, oldDomainsStr))

			if res.err != nil {
				return res.err
			}
		}

		// Add domains
		newDomainsStr := strings.Join(newDomains, " ")

		if len(newDomainsStr) > 0 {
			res := run(client, fmt.Sprintf("domains:add %s %s", appName, newDomainsStr))

			if res.err != nil {
				return res.err
			}
		}
	}

	if d.HasChange("buildpacks") {
		_, newBuildpacksI := d.GetChange("buildpacks")
		newBuildpacks := interfaceSliceToStrSlice(newBuildpacksI.([]interface{}))

		res := run(client, fmt.Sprintf("buildpacks:clear %s", appName))

		if res.err != nil {
			return res.err
		}
		app.Buildpacks = nil

		dokkuAppBuildpackAdd(appName, newBuildpacks, client)
	}

	if d.HasChange("ports") {
		oldPortListI, newPortListI := d.GetChange("ports")
		oldPortList := interfaceSliceToStrSlice(oldPortListI.(*schema.Set).List())
		newPortList := interfaceSliceToStrSlice(newPortListI.(*schema.Set).List())

		oldPortLookup := sliceToLookupMap(oldPortList)
		newPortLookup := sliceToLookupMap(newPortList)

		for _, p := range oldPortList {
			if _, ok := newPortLookup[p]; !ok {
				if len(p) > 0 {
					// the old port isn't in the new one, lets remove it
					res := run(client, fmt.Sprintf("%s %s %s", portRemoveCmd(), appName, p))

					if res.err != nil {
						return res.err
					}
				}
			}
		}

		for _, p := range newPortList {
			if _, ok := oldPortLookup[p]; !ok {
				if len(p) > 0 {
					// new port missing, lets add it
					res := run(client, fmt.Sprintf("%s %s %s", portAddCmd(), appName, p))

					if res.err != nil {
						return res.err
					}
				}
			}
		}
	}

	if d.HasChange("nginx_bind_address_ipv4") {
		_, newBindAddr := d.GetChange("nginx_bind_address_ipv4")
		dokkuAppNginxOptSet(appName, "bind-address-ipv4", newBindAddr.(string), client)
	}

	if d.HasChange("nginx_bind_address_ipv6") {
		_, newBindAddr := d.GetChange("nginx_bind_address_ipv6")
		dokkuAppNginxOptSet(appName, "bind-address-ipv6", newBindAddr.(string), client)
	}

	return nil
}
