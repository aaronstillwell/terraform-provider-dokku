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

// testAccProviderFactories serves the shared testAccProvider, so checks can use
// testAccProvider.Meta(). Use it instead of testAccProviders when a test config
// declares its own provider "dokku" block: the deprecated Providers field makes
// the SDK prepend an empty provider "dokku" {} block, which Terraform rejects as
// a duplicate.
var testAccProviderFactories = map[string]func() (*schema.Provider, error){
	"dokku": func() (*schema.Provider, error) { return testAccProvider, nil },
}

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
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
					provider "dokku" {
						ssh_cert = <<-EOF
%s
						EOF
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
		ProviderFactories: testAccProviderFactories,
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
		ProviderFactories: testAccProviderFactories,
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
