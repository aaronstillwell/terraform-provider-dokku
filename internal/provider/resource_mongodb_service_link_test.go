package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/melbahja/goph"
)

func TestAccMongodbServiceLink(t *testing.T) {
	serviceName := fmt.Sprintf("mongo-%s", acctest.RandString(10))
	appName := fmt.Sprintf("app-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		Providers:    testAccProviders,
		CheckDestroy: testMongodbServiceLinkDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "dokku_app" "test" {
	name = "%s"
}

resource "dokku_mongodb_service" "test" {
	name = "%s"
}

resource "dokku_mongodb_service_link" "test" {
	service = dokku_mongodb_service.test.name
	app = dokku_app.test.name
}
`, appName, serviceName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckMongodbServiceLinkExists("dokku_mongodb_service_link.test"),
				),
			},
		},
	})
}

func TestAccMongodbServiceLinkWithAlias(t *testing.T) {
	serviceName := fmt.Sprintf("mongo-%s", acctest.RandString(10))
	appName := fmt.Sprintf("app-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		Providers:    testAccProviders,
		CheckDestroy: testMongodbServiceLinkDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "dokku_app" "test" {
	name = "%s"
}

resource "dokku_mongodb_service" "test" {
	name = "%s"
}

resource "dokku_mongodb_service_link" "test" {
	service = dokku_mongodb_service.test.name
	app = dokku_app.test.name
	alias = "MONGODB"
}
`, appName, serviceName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckMongodbServiceLinkExists("dokku_mongodb_service_link.test"),
				),
			},
		},
	})
}

func TestAccMongodbServiceLinkWithQueryString(t *testing.T) {
	serviceName := fmt.Sprintf("mongo-%s", acctest.RandString(10))
	appName := fmt.Sprintf("app-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		Providers:    testAccProviders,
		CheckDestroy: testMongodbServiceLinkDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "dokku_app" "test" {
	name = "%s"
}

resource "dokku_mongodb_service" "test" {
	name = "%s"
}

resource "dokku_mongodb_service_link" "test" {
	service = dokku_mongodb_service.test.name
	app = dokku_app.test.name
	query_string = "?authSource=admin"
}
`, appName, serviceName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckMongodbServiceLinkExists("dokku_mongodb_service_link.test"),
				),
			},
		},
	})
}

func testAccCheckMongodbServiceLinkExists(n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]

		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("Service link ID not present")
		}

		sshClient := testAccProvider.Meta().(*goph.Client)

		service := rs.Primary.Attributes["service"]
		app := rs.Primary.Attributes["app"]

		// Create a new ResourceData from the ResourceState
		d := &schema.ResourceData{}
		d.SetId(rs.Primary.ID)
		d.Set("service", service)
		d.Set("app", app)
		if alias, ok := rs.Primary.Attributes["alias"]; ok {
			d.Set("alias", alias)
		}
		if queryString, ok := rs.Primary.Attributes["query_string"]; ok {
			d.Set("query_string", queryString)
		}

		err := serviceLinkRead(d, "mongodb", sshClient)

		if err != nil {
			return fmt.Errorf("Error reading mongodb service link %s -> %s: %v", service, app, err)
		}

		return nil
	}
}

func testMongodbServiceLinkDestroy(s *terraform.State) error {
	sshClient := testAccProvider.Meta().(*goph.Client)

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "dokku_mongodb_service_link" {
			continue
		}

		service := rs.Primary.Attributes["service"]
		app := rs.Primary.Attributes["app"]

		// Create a new ResourceData from the ResourceState
		d := &schema.ResourceData{}
		d.SetId(rs.Primary.ID)
		d.Set("service", service)
		d.Set("app", app)
		if alias, ok := rs.Primary.Attributes["alias"]; ok {
			d.Set("alias", alias)
		}
		if queryString, ok := rs.Primary.Attributes["query_string"]; ok {
			d.Set("query_string", queryString)
		}

		err := serviceLinkRead(d, "mongodb", sshClient)

		if err == nil {
			return fmt.Errorf("Dokku mongodb service link %s -> %s should not exist", service, app)
		}
	}

	return nil
}
