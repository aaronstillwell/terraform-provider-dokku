package dokku

import (
	"context"
	"fmt"
	"log"
	"net"
	"regexp"

	"github.com/blang/semver"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/melbahja/goph"
	"golang.org/x/crypto/ssh"
)

// Provider -
func Provider() *schema.Provider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"ssh_host": {
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("DOKKU_SSH_HOST", nil),
			},
			"ssh_user": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("DOKKU_SSH_USER", "dokku"),
			},
			"ssh_port": {
				Type:        schema.TypeInt,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("DOKKU_SSH_PORT", 22),
			},
			"ssh_cert": {
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("DOKKU_SSH_CERT", nil),
			},
			"fail_on_untested_version": {
				Type:        schema.TypeBool,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("DOKKU_FAIL_ON_UNTESTED_VERSION", true),
			},
			"skip_known_hosts_check": {
				Type:        schema.TypeBool,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("DOKKU_SKIP_KNOWN_HOSTS_CHECK", false),
			},
		},
		ResourcesMap: map[string]*schema.Resource{
			"dokku_app":                     resourceApp(),
			"dokku_postgres_service":        resourcePostgresService(),
			"dokku_postgres_service_link":   resourcePostgresServiceLink(),
			"dokku_redis_service":           resourceRedisService(),
			"dokku_redis_service_link":      resourceRedisServiceLink(),
			"dokku_mysql_service":           resourceMysqlService(),
			"dokku_mysql_service_link":      resourceMysqlServiceLink(),
			"dokku_clickhouse_service":      resourceClickhouseService(),
			"dokku_clickhouse_service_link": resourceClickhouseServiceLink(),
		},
		DataSourcesMap:       map[string]*schema.Resource{},
		ConfigureContextFunc: providerConfigure,
	}
}

func providerConfigure(ctx context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
	host := d.Get("ssh_host").(string)
	user := d.Get("ssh_user").(string)
	port := uint(d.Get("ssh_port").(int))
	certPath := d.Get("ssh_cert").(string)

	log.Printf("[DEBUG] establishing SSH connection\n")
	log.Printf("[DEBUG] host %v\n", host)
	log.Printf("[DEBUG] user %v\n", user)
	log.Printf("[DEBUG] port %v\n", port)
	log.Printf("[DEBUG] cert %v\n", certPath)
	log.Printf("[DEBUG] skip_known_hosts_check %v\n", d.Get("skip_known_hosts_check").(bool))

	var diags diag.Diagnostics

	auth, err := goph.Key(certPath, "")
	if err != nil {
		log.Printf("[ERROR]: %v", err)
		return nil, diag.Errorf("Could not find private key %s: %v", certPath, err)
	}

	// Check known hosts
	// https://github.com/melbahja/goph/blob/6258fe9f54bb1f738543020ade7ab22c1dd233d7/examples/goph/main.go#L75-L109
	verifyFn := func (host string, remote net.Addr, key ssh.PublicKey) error {
		// See https://github.com/aaronstillwell/terraform-provider-dokku/issues/15 - this option
		// has been implemented to support using the provider on hashicorp cloud platform
		if d.Get("skip_known_hosts_check").(bool) == false {
			hostFound, err := goph.CheckKnownHost(host, remote, key, "")

			// Host in known hosts but key mismatch
			if hostFound && err != nil {
				return err
			}

			// handshake because public key already exists.
			if hostFound && err == nil {
				return nil
			}
		} else {
			log.Printf("[WARN]: skip_known_hosts_check is set to true, no key verification will be run against the SSH host")
		}
		return goph.AddKnownHost(host, remote, key, "")
	}

	sshConfig := &goph.Config{
		Auth:     auth,
		Addr:     host,
		Port:     port,
		User:     user,
		Callback: verifyFn,
	}

	client, err := goph.NewConn(sshConfig)
	if err != nil {
		log.Printf("[ERROR]: %v", err)
		return nil, diag.Errorf("Could not establish SSH connection: %v", err)
	}

	res := run(client, "version")

	re := regexp.MustCompile("[0-9]+\\.[0-9]+\\.[0-9]+")
	found := re.FindString(res.stdout)

	hostVersion, err := semver.Parse(string(found))

	log.Printf("[DEBUG] host version %v", hostVersion)

	testedVersions := ">=0.24.0 <0.31.0"
	testedErrMsg := fmt.Sprintf("This provider has not been tested against Dokku version %s. Tested version range: %s", string(found), testedVersions)

	if err == nil {
		compat, _ := semver.ParseRange(testedVersions)

		if !compat(hostVersion) {

			log.Printf("[DEBUG] fail_on_untested_version: %v", d.Get("fail_on_untested_version").(bool))

			if d.Get("fail_on_untested_version").(bool) {
				return client, diag.Errorf(testedErrMsg)
			}
			warn := diag.Diagnostic{
				Severity: diag.Warning,
				Summary:  testedErrMsg,
			}
			diags = append(diags, warn)
			return client, diags
		}
	} else {
		return client, diag.Errorf("Could not detect dokku version - tested version range: %s", testedVersions)
	}

	return client, diags
}
