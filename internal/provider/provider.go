package provider

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

var DOKKU_VERSION semver.Version

// Provider -
func Provider() *schema.Provider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"ssh_host": {
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("DOKKU_SSH_HOST", nil),
				Description: "The hostname of your Dokku server. Can be set via DOKKU_SSH_HOST environment variable.",
			},
			"ssh_user": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("DOKKU_SSH_USER", "dokku"),
				Description: "The SSH user to connect to your Dokku server. Defaults to 'dokku'. Can be set via DOKKU_SSH_USER environment variable.",
			},
			"ssh_port": {
				Type:        schema.TypeInt,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("DOKKU_SSH_PORT", 22),
				Description: "The SSH port of your Dokku server. Defaults to 22. Can be set via DOKKU_SSH_PORT environment variable.",
			},
			"ssh_cert": {
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("DOKKU_SSH_CERT", nil),
				Description: "Either a path to the SSH private key for connecting to your Dokku server OR the source for an SSH key directly. Can be set via DOKKU_SSH_CERT environment variable.",
			},
			"ssh_passphrase": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("DOKKU_SSH_PASSPHRASE", nil),
				Description: "An optional passphrase to be used in conjunction with the provided SSH key.",
			},
			"fail_on_untested_version": {
				Type:        schema.TypeBool,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("DOKKU_FAIL_ON_UNTESTED_VERSION", true),
				Description: "Whether to fail if the Dokku version has not been tested with this provider. Defaults to true. Can be set via DOKKU_FAIL_ON_UNTESTED_VERSION environment variable.",
			},
			"skip_known_hosts_check": {
				Type:        schema.TypeBool,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("DOKKU_SKIP_KNOWN_HOSTS_CHECK", false),
				Description: "Whether to skip SSH known hosts verification. Defaults to false. Can be set via DOKKU_SKIP_KNOWN_HOSTS_CHECK environment variable.",
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
	ssh_cert := d.Get("ssh_cert").(string)
	ssh_passphrase := d.Get("ssh_passphrase").(string)

	var auth goph.Auth
	var err error
	if ssh_passphrase != "" {
		_, err = ssh.ParsePrivateKeyWithPassphrase([]byte(ssh_cert), []byte(ssh_passphrase))
	} else {
		_, err = ssh.ParsePrivateKey([]byte(ssh_cert))
	}

	if err != nil {
		log.Printf("[DEBUG] attempting to load SSH cert from file path %s\n", ssh_cert)
		log.Printf("[DEBUG] %v\n", err)
		// could not parse a key directly, try it as a filepath
		var err error
		auth, err = goph.Key(ssh_cert, ssh_passphrase)
		if err != nil {
			return nil, diag.Errorf("Attempted to load private key from file but failed")
		}
	}

	if auth == nil {
		log.Printf("[DEBUG] SSH cert looks like its being provided inline\n")
		var err error
		auth, err = goph.RawKey(ssh_cert, ssh_passphrase)

		if err != nil {
			// could not proceed with inline ssh cert
			return nil, diag.Errorf("Could not auth with inline SSH key: %v", err)
		}
	}

	log.Printf("[DEBUG] establishing SSH connection\n")
	log.Printf("[DEBUG] host %v\n", host)
	log.Printf("[DEBUG] user %v\n", user)
	log.Printf("[DEBUG] port %v\n", port)
	log.Printf("[DEBUG] skip_known_hosts_check %v\n", d.Get("skip_known_hosts_check").(bool))

	var diags diag.Diagnostics

	// Check known hosts
	// https://github.com/melbahja/goph/blob/6258fe9f54bb1f738543020ade7ab22c1dd233d7/examples/goph/main.go#L75-L109
	verifyFn := func(host string, remote net.Addr, key ssh.PublicKey) error {
		// See https://github.com/aaronstillwell/terraform-provider-dokku/issues/15 - this option
		// has been implemented to support using the provider on Terraform Cloud
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

	// Check for 127 status code... suggests that we're not authenticating
	// with a dokku user (see https://github.com/aaronstillwell/terraform-provider-dokku/issues/1)
	if res.status == 127 {
		log.Printf("[ERROR] must use a dokku user for authentication, see the docs")
		return nil, diag.Errorf("[ERROR] must use a dokku user for authentication, see the docs")
	}

	re := regexp.MustCompile("[0-9]+\\.[0-9]+\\.[0-9]+")
	found := re.FindString(res.stdout)

	hostVersion, err := semver.Parse(string(found))

	DOKKU_VERSION = hostVersion

	log.Printf("[DEBUG] host version %v", hostVersion)

	testedVersions := ">=0.30.0 <0.36.0"
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
