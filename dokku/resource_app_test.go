package dokku

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/melbahja/goph"
)

func TestAccDokkuApp(t *testing.T) {
	appName := fmt.Sprintf("test-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		Providers:    testAccProviders,
		CheckDestroy: testAccDokkuAppDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "dokku_app" "test" {
  name   = "%s"
}`, appName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDokkuAppExists("dokku_app.test"),
				),
			},
		},
	})
}

func TestAccDokkuAppConfigVars(t *testing.T) {
	appName := fmt.Sprintf("test-config-vars-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		Providers:    testAccProviders,
		CheckDestroy: testAccDokkuAppDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "dokku_app" "test" {
	name = "%s"
	config_vars = {
		FOO = "BAR"
	}
}				
`, appName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDokkuAppExists("dokku_app.test"),
					testAccCheckDokkuAppConfigVar("dokku_app.test", "FOO", "BAR"),
				),
			},
		},
	})
}

func TestAccDokkuAppDomain(t *testing.T) {
	appName := fmt.Sprintf("test-domain-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		Providers:    testAccProviders,
		CheckDestroy: testAccDokkuAppDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "dokku_app" "test" {
	name = "%s"
	domains = ["test.dokku.me"]
}				
`, appName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDokkuAppExists("dokku_app.test"),
					testAccCheckDokkuDomain("dokku_app.test", "test.dokku.me"),
				),
			},
		},
	})
}

func TestCompleteApp(t *testing.T) {
	appName := fmt.Sprintf("test-app-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		Providers:    testAccProviders,
		CheckDestroy: testAccDokkuAppDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "dokku_app" "test" {
	name = "%s"
	config_vars = {
		FOO2 = "BAR:FOO"
	}
	domains = ["test.dokku.me2"]
}				
`, appName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDokkuAppExists("dokku_app.test"),
					testAccCheckDokkuAppConfigVar("dokku_app.test", "FOO2", "BAR:FOO"),
					testAccCheckDokkuDomain("dokku_app.test", "test.dokku.me2"),
				),
			},
		},
	})
}

//
func testAccCheckDokkuAppExists(n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]

		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("App ID not present")
		}

		sshClient := testAccProvider.Meta().(*goph.Client)

		_, err := dokkuAppRetrieve(rs.Primary.ID, sshClient)

		if err != nil {
			return fmt.Errorf("Error retrieving app info")
		}

		return nil
	}
}

//
func testAccCheckDokkuAppConfigVar(n string, varName string, varValue string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]

		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		sshClient := testAccProvider.Meta().(*goph.Client)

		app, err := dokkuAppRetrieve(rs.Primary.ID, sshClient)

		if err != nil {
			return fmt.Errorf("Error retrieving app info")
		}

		val, ok := app.ConfigVars[varName]

		if !ok {
			return fmt.Errorf("Config var %s not found", varName)
		}

		if val != varValue {
			return fmt.Errorf("Config var expected to be %s, was %s", varValue, val)
		}

		return nil
	}
}

func testAccCheckDokkuDomain(n string, domains ...string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]

		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		sshClient := testAccProvider.Meta().(*goph.Client)

		app, err := dokkuAppRetrieve(rs.Primary.ID, sshClient)

		if err != nil {
			return fmt.Errorf("Error retrieving app info")
		}

		if len(app.Domains) != len(domains) {
			return fmt.Errorf("Expected %d domains, got %d", len(domains), len(app.Domains))
		}

		for k, _ := range app.Domains {
			if app.Domains[k] != domains[k] {
				return fmt.Errorf("Expected domain %s, got %s", domains[k], app.Domains[k])
			}
		}

		return nil
	}
}

//
func testAccDokkuAppDestroy(s *terraform.State) error {
	sshClient := testAccProvider.Meta().(*goph.Client)

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "dokku_app" {
			continue
		}

		app, _ := dokkuAppRetrieve(rs.Primary.ID, sshClient)

		if app.Id != "" {
			return fmt.Errorf("Dokku app %s should not exist", rs.Primary.ID)
		}
	}

	return nil
}
