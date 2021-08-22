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
