package provider

import (
	"fmt"
	"log"
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
					testAccCheckDokkuAppDomain("dokku_app.test", "test.dokku.me"),
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
		FOO3 = "FOO BAR"
	}
	domains = ["test.dokku.me2"]
}				
`, appName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDokkuAppExists("dokku_app.test"),
					testAccCheckDokkuAppConfigVar("dokku_app.test", "FOO2", "BAR:FOO"),
					testAccCheckDokkuAppConfigVar("dokku_app.test", "FOO3", "FOO BAR"),
					testAccCheckDokkuAppDomain("dokku_app.test", "test.dokku.me2"),
				),
			},
		},
	})
}

func TestRenameApp(t *testing.T) {
	appName := fmt.Sprintf("test-app-%s", acctest.RandString(10))
	newName := fmt.Sprintf("test-app-new-%s", acctest.RandString(10))

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
					testAccCheckDokkuAppDomain("dokku_app.test", "test.dokku.me2"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "dokku_app" "test" {
	name = "%s"
	config_vars = {
		FOO2 = "BAR:FOO"
	}
	domains = ["test.dokku.me2"]
}
`, newName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDokkuAppExists("dokku_app.test"),
					testAccCheckDokkuAppConfigVar("dokku_app.test", "FOO2", "BAR:FOO"),
					testAccCheckDokkuAppDomain("dokku_app.test", "test.dokku.me2"),
				),
			},
		},
	})
}

func TestSetAppConfigVars(t *testing.T) {
	appName := fmt.Sprintf("test-app-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		Providers:    testAccProviders,
		CheckDestroy: testAccDokkuAppDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "dokku_app" "test" {
	name = "%s"
}				
`, appName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDokkuAppExists("dokku_app.test"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "dokku_app" "test" {
	name = "%s"
	config_vars = {
		FOO2 = "BAR:FOO"
	}
}`, appName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDokkuAppExists("dokku_app.test"),
					testAccCheckDokkuAppConfigVar("dokku_app.test", "FOO2", "BAR:FOO"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "dokku_app" "test" {
	name = "%s"
	config_vars = {
		FOO3 = "FOO BAR"
	}
}`, appName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDokkuAppExists("dokku_app.test"),
					testAccCheckDokkuAppConfigVar("dokku_app.test", "FOO3", "FOO BAR"),
				),
			},
		},
	})
}

func TestUnsetAppConfigVar(t *testing.T) {
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
}				
`, appName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDokkuAppExists("dokku_app.test"),
					testAccCheckDokkuAppConfigVar("dokku_app.test", "FOO2", "BAR:FOO"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "dokku_app" "test" {
	name = "%s"
}`, appName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDokkuAppExists("dokku_app.test"),
					testAccCheckDokkuAppConfigVarUnset("dokku_app.test", "FOO2"),
				),
			},
		},
	})
}

func TestSetAppDomain(t *testing.T) {
	appName := fmt.Sprintf("test-app-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		Providers:    testAccProviders,
		CheckDestroy: testAccDokkuAppDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "dokku_app" "test" {
	name = "%s"
}
`, appName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDokkuAppExists("dokku_app.test"),
					testAccCheckDokkuAppDomainsLen("dokku_app.test", 0),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "dokku_app" "test" {
	name = "%s"
	domains = ["test.dokku.me"]
}
`, appName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDokkuAppExists("dokku_app.test"),
					testAccCheckDokkuAppDomainsLen("dokku_app.test", 1),
					testAccCheckDokkuAppDomain("dokku_app.test", "test.dokku.me"),
				),
			},
		},
	})
}

func TestUnsetAppDomain(t *testing.T) {
	appName := fmt.Sprintf("test-app-%s", acctest.RandString(10))

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
					testAccCheckDokkuAppDomainsLen("dokku_app.test", 1),
					testAccCheckDokkuAppDomain("dokku_app.test", "test.dokku.me"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "dokku_app" "test" {
	name = "%s"
}
`, appName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDokkuAppExists("dokku_app.test"),
					testAccCheckDokkuAppDomainsLen("dokku_app.test", 0),
				),
			},
		},
	})
}

func TestAppBuildpacks(t *testing.T) {
	appName := fmt.Sprintf("test-buildpack-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		Providers:    testAccProviders,
		CheckDestroy: testAccDokkuAppDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "dokku_app" "test" {
	name = "%s"
	buildpacks = ["https://github.com/heroku/heroku-buildpack-nodejs.git"]
}
`, appName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDokkuAppExists("dokku_app.test"),
					testAccCheckDokkuBuildpacks("dokku_app.test", "https://github.com/heroku/heroku-buildpack-nodejs.git"),
				),
			},
		},
	})
}

func TestAppAddBuildpack(t *testing.T) {
	appName := fmt.Sprintf("test-buildpack-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		Providers:    testAccProviders,
		CheckDestroy: testAccDokkuAppDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "dokku_app" "test" {
	name = "%s"
}
`, appName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDokkuAppExists("dokku_app.test"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "dokku_app" "test" {
	name = "%s"
  buildpacks = [
    "https://github.com/heroku/heroku-buildpack-nodejs.git",
    "https://github.com/heroku/heroku-buildpack-ruby.git"
  ]
}
`, appName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDokkuAppExists("dokku_app.test"),
					testAccCheckDokkuBuildpacks("dokku_app.test", "https://github.com/heroku/heroku-buildpack-nodejs.git", "https://github.com/heroku/heroku-buildpack-ruby.git"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "dokku_app" "test" {
	name = "%s"
	buildpacks = [
		"https://github.com/heroku/heroku-buildpack-ruby.git",
    "https://github.com/heroku/heroku-buildpack-nodejs.git"
  ]
}
`, appName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDokkuAppExists("dokku_app.test"),
					testAccCheckDokkuBuildpacks("dokku_app.test", "https://github.com/heroku/heroku-buildpack-ruby.git", "https://github.com/heroku/heroku-buildpack-nodejs.git"),
				),
			},
		},
	})
}

func TestAppRemoveBuildpack(t *testing.T) {
	appName := fmt.Sprintf("test-buildpack-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		Providers:    testAccProviders,
		CheckDestroy: testAccDokkuAppDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "dokku_app" "test" {
	name = "%s"
  buildpacks = [
    "https://github.com/heroku/heroku-buildpack-nodejs.git",
    "https://github.com/heroku/heroku-buildpack-ruby.git"
  ]
}
`, appName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDokkuAppExists("dokku_app.test"),
					testAccCheckDokkuBuildpacks("dokku_app.test", "https://github.com/heroku/heroku-buildpack-nodejs.git", "https://github.com/heroku/heroku-buildpack-ruby.git"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "dokku_app" "test" {
	name = "%s"
	buildpacks = [
		"https://github.com/heroku/heroku-buildpack-ruby.git"
  ]
}
`, appName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDokkuAppExists("dokku_app.test"),
					testAccCheckDokkuBuildpacks("dokku_app.test", "https://github.com/heroku/heroku-buildpack-ruby.git"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "dokku_app" "test" {
	name = "%s"
}
`, appName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDokkuAppExists("dokku_app.test"),
					testAccCheckDokkuBuildpacks("dokku_app.test"),
				),
			},
		},
	})
}

func TestAppPort(t *testing.T) {
	appName := fmt.Sprintf("test-ports-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		Providers:    testAccProviders,
		CheckDestroy: testAccDokkuAppDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "dokku_app" "test" {
	name = "%s"
	ports = ["tcp:25:3000"]
}
`, appName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDokkuAppExists("dokku_app.test"),
					testAccCheckDokkuAppPortsExist("dokku_app.test", "tcp:25:3000"),
				),
			},
		},
	})
}

func TestAppAddPort(t *testing.T) {
	appName := fmt.Sprintf("test-ports-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		Providers:    testAccProviders,
		CheckDestroy: testAccDokkuAppDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "dokku_app" "test" {
	name = "%s"
}
`, appName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDokkuAppExists("dokku_app.test"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "dokku_app" "test" {
	name = "%s"
	ports = ["tcp:25:3000"]
}
`, appName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDokkuAppExists("dokku_app.test"),
					testAccCheckDokkuAppPortsExist("dokku_app.test", "tcp:25:3000"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "dokku_app" "test" {
	name = "%s"
	ports = [
		"tcp:25:3000",
		"tcp:26:3001"
	]
}
`, appName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDokkuAppExists("dokku_app.test"),
					testAccCheckDokkuAppPortsExist("dokku_app.test", "tcp:25:3000", "tcp:26:3001"),
				),
			},
		},
	})
}

func TestAppRemovePort(t *testing.T) {
	appName := fmt.Sprintf("test-ports-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		Providers:    testAccProviders,
		CheckDestroy: testAccDokkuAppDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "dokku_app" "test" {
	name = "%s"
	ports = [
		"tcp:25:3000",
		"tcp:26:3001"
	]
}
`, appName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDokkuAppExists("dokku_app.test"),
					testAccCheckDokkuAppPortsExist("dokku_app.test", "tcp:25:3000", "tcp:26:3001"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "dokku_app" "test" {
	name = "%s"
	ports = ["tcp:25:3000"]
}
`, appName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDokkuAppExists("dokku_app.test"),
					testAccCheckDokkuAppPortsExist("dokku_app.test", "tcp:25:3000"),
					testAccCheckDokkuAppPortsDontExist("dokku_app.test", "tcp:26:3001"),
				),
			},
			{
				Config: fmt.Sprintf(`
				resource "dokku_app" "test" {
					name = "%s"
				}
				`, appName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDokkuAppExists("dokku_app.test"),
					testAccCheckDokkuAppPortsDontExist("dokku_app.test", "tcp:25:3000", "tcp:26:3001"),
				),
			},
		},
	})
}

func TestAppNginxIpv4Address(t *testing.T) {
	appName := fmt.Sprintf("test-nginxip4-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		Providers:    testAccProviders,
		CheckDestroy: testAccDokkuAppDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "dokku_app" "test" {
	name = "%s"
	nginx_bind_address_ipv4 = "192.168.1.1"
}
`, appName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDokkuAppExists("dokku_app.test"),
					testAccCheckDokkuAppNginxIpv4Addr("dokku_app.test", "192.168.1.1"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "dokku_app" "test" {
	name = "%s"
}
`, appName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDokkuAppExists("dokku_app.test"),
					testAccCheckDokkuAppNginxIpv4Addr("dokku_app.test", "0.0.0.0"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "dokku_app" "test" {
	name = "%s"
	nginx_bind_address_ipv4 = "1.1.1.1"
}
`, appName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDokkuAppExists("dokku_app.test"),
					testAccCheckDokkuAppNginxIpv4Addr("dokku_app.test", "1.1.1.1"),
				),
			},
		},
	})
}

func TestAppNginxIpv6Address(t *testing.T) {
	appName := fmt.Sprintf("test-nginxip6-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		Providers:    testAccProviders,
		CheckDestroy: testAccDokkuAppDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "dokku_app" "test" {
	name = "%s"
	nginx_bind_address_ipv6 = "2001:0db8:0000:0000:0000:ff00:0042:8329"
}
`, appName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDokkuAppExists("dokku_app.test"),
					testAccCheckDokkuAppNginxIpv6Addr("dokku_app.test", "2001:0db8:0000:0000:0000:ff00:0042:8329"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "dokku_app" "test" {
	name = "%s"
}
`, appName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDokkuAppExists("dokku_app.test"),
					testAccCheckDokkuAppNginxIpv6Addr("dokku_app.test", "::"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "dokku_app" "test" {
	name = "%s"
	nginx_bind_address_ipv6 = "2001:0db8:0000:0000:0000:ff00:0042:9000"
}
`, appName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDokkuAppExists("dokku_app.test"),
					testAccCheckDokkuAppNginxIpv6Addr("dokku_app.test", "2001:0db8:0000:0000:0000:ff00:0042:9000"),
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

func testAccCheckDokkuAppConfigVarUnset(n string, varName string) resource.TestCheckFunc {
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

		_, ok = app.ConfigVars[varName]

		if ok {
			return fmt.Errorf("Config var %s was found but expected to be unset", varName)
		}

		return nil
	}
}

func testAccCheckDokkuAppDomain(n string, domains ...string) resource.TestCheckFunc {
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

func testAccCheckDokkuAppDomainsLen(n string, nOfDomains int) resource.TestCheckFunc {
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

		if len(app.Domains) != nOfDomains {
			return fmt.Errorf("Expected %d domains, got %d", nOfDomains, len(app.Domains))
		}

		return nil
	}
}

func testAccCheckDokkuBuildpacks(n string, buildpacks ...string) resource.TestCheckFunc {
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

		if len(app.Buildpacks) != len(buildpacks) {
			return fmt.Errorf("Expected %d buildpacks, got %d", len(buildpacks), len(app.Domains))
		}

		for k, _ := range app.Buildpacks {
			if app.Buildpacks[k] != buildpacks[k] {
				return fmt.Errorf("Expected buildpack %s, got %s", buildpacks[k], app.Buildpacks[k])
			}
		}

		return nil
	}
}

func testAccCheckDokkuAppPortsExist(n string, ports ...string) resource.TestCheckFunc {
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

		var portsFound []string
		portLookup := sliceToLookupMap(ports)

		log.Printf("[DEBUG] %v", app.Ports)

		for _, p := range app.Ports {
			if _, ok := portLookup[p]; ok {
				portsFound = append(portsFound, p)
				delete(portLookup, p)
			}
		}

		if len(portLookup) > 0 {
			return fmt.Errorf("%d ports expected, %d ports found. Ports not found: %v\n", len(ports), len(portLookup), portLookup)
		}

		return nil
	}
}

func testAccCheckDokkuAppPortsDontExist(n string, ports ...string) resource.TestCheckFunc {
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

		portLookup := sliceToLookupMap(ports)
		for _, p := range app.Ports {
			if _, ok := portLookup[p]; ok {
				return fmt.Errorf("Port %s should not exist", p)
			}
		}

		return nil
	}
}

//
func testAccCheckDokkuAppNginxIpv4Addr(n string, ip string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]

		if !ok {
			return fmt.Errorf("Not found %s", n)
		}

		sshClient := testAccProvider.Meta().(*goph.Client)

		app, err := dokkuAppRetrieve(rs.Primary.ID, sshClient)

		if err != nil {
			return fmt.Errorf("Error retrieving app info")
		}

		if app.NginxBindAddressIpv4 != ip {
			return fmt.Errorf("nginx_bind_address_ipv4 was %s, expected %s", app.NginxBindAddressIpv4, ip)
		}

		return nil
	}
}

//
func testAccCheckDokkuAppNginxIpv6Addr(n string, ip string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]

		if !ok {
			return fmt.Errorf("Not found %s", n)
		}

		sshClient := testAccProvider.Meta().(*goph.Client)

		app, err := dokkuAppRetrieve(rs.Primary.ID, sshClient)

		if err != nil {
			return fmt.Errorf("Error retrieving app info")
		}

		if app.NginxBindAddressIpv6 != ip {
			return fmt.Errorf("nginx_bind_address_ipv6 was %s, expected %s", app.NginxBindAddressIpv6, ip)
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
