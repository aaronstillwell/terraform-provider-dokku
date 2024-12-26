package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

var testAccProviders map[string]*schema.Provider
var testAccProvider *schema.Provider

func init() {
	testAccProvider = Provider()
	testAccProviders = map[string]*schema.Provider{
		"dokku": testAccProvider,
	}
}

func TestAccInlineSshKey(t *testing.T) {
	certPath := os.Getenv("DOKKU_SSH_CERT")
	content, _ := os.ReadFile(certPath)

	os.Setenv("DOKKU_SSH_CERT", string(content))

	t.Cleanup(func() {
		os.Setenv("DOKKU_SSH_CERT", certPath)
	})

	resource.Test(t, resource.TestCase{
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: `
					resource "dokku_app" "test" {
						name = "test-connection"
					}
				`,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDokkuAppExists("dokku_app.test"),
				),
			},
		},
	})
}

func TestAccInlineSshKeyWithPassphrase(t *testing.T) {
	certPath := os.Getenv("DOKKU_SSH_CERT_WITH_PASSPHRASE")
	sshKey, _ := os.ReadFile(certPath)

	resource.Test(t, resource.TestCase{
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
					provider "dokku" {
						ssh_cert = "%s"
						ssh_passphrase = "foobar"
					}

					resource "dokku_app" "test" {
						name = "test-connection"
					}
				`, sshKey),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDokkuAppExists("dokku_app.test"),
				),
			},
		},
	})
}

func TestAccSshKeyOnDiskWithPassphrase(t *testing.T) {
	certPath := os.Getenv("DOKKU_SSH_CERT_WITH_PASSPHRASE")

	resource.Test(t, resource.TestCase{
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
					provider "dokku" {
						ssh_cert = "%s"
						ssh_passphrase = "foobar"
					}

					resource "dokku_app" "test" {
						name = "test-connection"
					}
				`, certPath),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDokkuAppExists("dokku_app.test"),
				),
			},
		},
	})
}

func TestAccSshKeyOnDiskWithPassphraseFromEnv(t *testing.T) {
	certPath := os.Getenv("DOKKU_SSH_CERT_WITH_PASSPHRASE")

	os.Setenv("DOKKU_SSH_PASSPHRASE", "foobar")

	t.Cleanup(func() {
		os.Unsetenv("DOKKU_SSH_PASSPHRASE")
	})

	resource.Test(t, resource.TestCase{
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
					provider "dokku" {
						ssh_cert = "%s"
					}

					resource "dokku_app" "test" {
						name = "test-connection"
					}
				`, certPath),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDokkuAppExists("dokku_app.test"),
				),
			},
		},
	})
}
