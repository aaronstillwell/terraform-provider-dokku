package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
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
					testAccServiceLinkExists("mongo", serviceName, appName),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "dokku_app" "test" {
	name = "%s"
}

resource "dokku_mongodb_service" "test" {
	name = "%s"
}
`, appName, serviceName),
				Check: resource.ComposeTestCheckFunc(
					testAccServiceLinkNotExists("mongo", serviceName, appName),
				),
			},
		},
	})
}

func testMongodbServiceLinkDestroy(s *terraform.State) error {
	sshClient := testAccProvider.Meta().(*goph.Client)

	for _, rs := range s.RootModule().Resources {
		if rs.Type == "dokku_mongodb_service" {
			service := NewDokkuMongodbService(rs.Primary.ID)
			err := dokkuMongodbRead(service, sshClient)

			if err != nil {
				return fmt.Errorf("Could not read Mongodb service %s", rs.Primary.ID)
			}

			if service.Id != "" {
				return fmt.Errorf("Mongodb service %s should not exist", rs.Primary.ID)
			}
		}
	}

	return nil
}
