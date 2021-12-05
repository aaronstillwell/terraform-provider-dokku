package dokku

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/melbahja/goph"
)

func TestAccMysqlServiceLink(t *testing.T) {
	appName := fmt.Sprintf("mysql-app-%s", acctest.RandString(10))
	serviceName := fmt.Sprintf("mysql-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		Providers:    testAccProviders,
		CheckDestroy: testMysqlServiceLinkDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "dokku_app" "test" {
	name = "%s"
}

resource "dokku_mysql_service" "test" {
	name = "%s"
}

resource "dokku_mysql_service_link" "test" {
	app = dokku_app.test.name
	service = dokku_mysql_service.test.name
}
`, appName, serviceName),
				Check: resource.ComposeTestCheckFunc(
					testAccMysqlServiceIsLinked(serviceName, appName),
				),
			},
			{
				Config: fmt.Sprintf(`
	resource "dokku_app" "test" {
		name = "%s"
	}
	
	resource "dokku_mysql_service" "test" {
		name = "%s"
	}
	`, appName, serviceName),
				Check: resource.ComposeTestCheckFunc(
					testAccMysqlServiceIsNotLinked(serviceName, appName),
				),
			},
		},
	})
}

func testAccMysqlServiceIsLinked(serviceName string, appName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		sshClient := testAccProvider.Meta().(*goph.Client)

		out := run(sshClient, fmt.Sprintf("mysql:linked %s %s", serviceName, appName))

		if out.err != nil {
			return fmt.Errorf("service %s not linked to app %s - %v", serviceName, appName, out.err)
		}
		return nil
	}
}

func testAccMysqlServiceIsNotLinked(serviceName string, appName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		sshClient := testAccProvider.Meta().(*goph.Client)

		out := run(sshClient, fmt.Sprintf("mysql:linked %s %s", serviceName, appName))

		if out.err == nil {
			return fmt.Errorf("service %s still linked to app %s - %v", serviceName, appName, out.err)
		}
		return nil
	}
}

// Shouldn't really need to be explicit about the link being destroyed - if
// app and service both gone then the link cannot exist
func testMysqlServiceLinkDestroy(s *terraform.State) error {
	sshClient := testAccProvider.Meta().(*goph.Client)

	for _, rs := range s.RootModule().Resources {
		if rs.Type == "dokku_app" {
			app, _ := dokkuAppRetrieve(rs.Primary.ID, sshClient)

			if app.Id != "" {
				return fmt.Errorf("Dokku app %s should not exist", rs.Primary.ID)
			}
		} else if rs.Type == "dokku_mysql_service" {
			mysql := NewMysqlService(rs.Primary.ID)
			err := dokkuMysqlRead(mysql, sshClient)

			if err != nil {
				return fmt.Errorf("Could not read MySQL service %s", rs.Primary.ID)
			}

			if mysql.Id != "" {
				return fmt.Errorf("Mysql service %s should not exist", rs.Primary.ID)
			}
		}
	}

	return nil
}
