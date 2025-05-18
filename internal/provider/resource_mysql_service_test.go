package provider

import (
	"fmt"
	"slices"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/melbahja/goph"
)

func TestAccMysqlService(t *testing.T) {
	serviceName := fmt.Sprintf("mysql-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		Providers:    testAccProviders,
		CheckDestroy: testMysqlServiceDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "dokku_mysql_service" "test" {
	name = "%s"
}
`, serviceName),
				Check: testMysqlServiceExists("dokku_mysql_service.test"),
			},
		},
	})
}

func TestAccMysqlServiceImageVersion(t *testing.T) {
	serviceName := fmt.Sprintf("mysql-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		Providers:    testAccProviders,
		CheckDestroy: testMysqlServiceDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "dokku_mysql_service" "test" {
	name = "%s"
	image_version = "5.7.36"
}
`, serviceName),
				Check: resource.ComposeTestCheckFunc(
					testMysqlServiceExists("dokku_mysql_service.test"),
					testMysqlImageVersion("dokku_mysql_service.test", "5.7.36"),
				),
			},
		},
	})
}

func TestAccMysqlServiceUpdate(t *testing.T) {
	serviceName := fmt.Sprintf("mysql-%s", acctest.RandString(10))
	newServiceName := fmt.Sprintf("mysql-new-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		Providers:    testAccProviders,
		CheckDestroy: testMysqlServiceDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "dokku_mysql_service" "test" {
	name = "%s"
}
`, serviceName),
				Check: testMysqlServiceExists("dokku_mysql_service.test"),
			},
			{
				Config: fmt.Sprintf(`
resource "dokku_mysql_service" "test" {
	name = "%s"
	image_version = "5.7.36"
}
`, newServiceName),
				Check: resource.ComposeTestCheckFunc(
					testMysqlServiceExists("dokku_mysql_service.test"),
					testMysqlImageVersion("dokku_mysql_service.test", "5.7.36"),
					testMysqlServiceName("dokku_mysql_service.test", newServiceName),
				),
			},
		},
	})
}

func TestAccMysqlServiceExposedOn(t *testing.T) {
	serviceName := fmt.Sprintf("mysql-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		Providers:    testAccProviders,
		CheckDestroy: testMysqlServiceDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "dokku_mysql_service" "test" {
	name = "%s"
}
`, serviceName),
				Check: resource.ComposeTestCheckFunc(
					testMysqlServiceExists("dokku_mysql_service.test"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "dokku_mysql_service" "test" {
	name = "%s"
	expose_on = "0.0.0.0:8585"
}
`, serviceName),
				Check: resource.ComposeTestCheckFunc(
					testMysqlServiceExists("dokku_mysql_service.test"),
					testMysqlExposed("dokku_mysql_service.test", true, "0.0.0.0:8585"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "dokku_mysql_service" "test" {
	name = "%s"
}
`, serviceName),
				Check: resource.ComposeTestCheckFunc(
					testMysqlServiceExists("dokku_mysql_service.test"),
					testMysqlExposed("dokku_mysql_service.test", false, ""),
				),
			},
		},
	})
}

func testMysqlServiceExists(n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]

		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("Service ID not present")
		}

		sshClient := testAccProvider.Meta().(*goph.Client)

		service := NewMysqlService(rs.Primary.ID)
		err := dokkuMysqlRead(service, sshClient)

		if err != nil {
			return fmt.Errorf("Error reading mysql resource %s", rs.Primary.ID)
		}

		if service.Id == "" {
			return fmt.Errorf("Service %s was not created", rs.Primary.ID)
		}

		return nil
	}
}

func testMysqlServiceName(n string, name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]

		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("Service ID not present")
		}

		sshClient := testAccProvider.Meta().(*goph.Client)

		service := NewMysqlService(rs.Primary.ID)
		err := dokkuMysqlRead(service, sshClient)

		if err != nil {
			return fmt.Errorf("Error reading mysql resource %s", rs.Primary.ID)
		}

		if service.Id == "" {
			return fmt.Errorf("Service %s was not created", rs.Primary.ID)
		}

		if service.Name != name {
			return fmt.Errorf("Service name was %s, expected %s", service.Name, name)
		}

		return nil
	}
}

func testMysqlImageVersion(n string, version string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]

		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("Service ID not present")
		}

		sshClient := testAccProvider.Meta().(*goph.Client)

		service := NewMysqlService(rs.Primary.ID)
		err := dokkuMysqlRead(service, sshClient)

		if err != nil {
			return fmt.Errorf("Error reading mysql resource %s", rs.Primary.ID)
		}

		if service.ImageVersion != version {
			return fmt.Errorf("Image version expected to be %s, got %s", version, service.ImageVersion)
		}

		return nil
	}
}

func testMysqlExposed(n string, isExposed bool, host string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]

		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("Service ID not present")
		}

		sshClient := testAccProvider.Meta().(*goph.Client)

		service := NewMysqlService(rs.Primary.ID)
		err := dokkuMysqlRead(service, sshClient)

		if err != nil {
			return fmt.Errorf("Error reading mysql resource %s", rs.Primary.ID)
		}

		if isExposed {
			if len(service.Exposed) != 1 || !slices.Contains(service.Exposed, host) {
				return fmt.Errorf("mysql was not exposed as expected, returned %s", service.Exposed)
			}
		} else {
			if service.Exposed != nil {
				return fmt.Errorf("Service was exposed unexpectedly, returned %s", service.Exposed)
			}
		}

		return nil
	}
}

func testMysqlServiceDestroy(s *terraform.State) error {
	sshClient := testAccProvider.Meta().(*goph.Client)

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "dokku_mysql_service" {
			continue
		}

		service := NewMysqlService(rs.Primary.ID)
		err := dokkuMysqlRead(service, sshClient)

		if err != nil {
			return fmt.Errorf("Dokku mysql service %s could not be read: %v", rs.Primary.ID, err)
		}

		if service.Id != "" {
			return fmt.Errorf("Dokku mysql service %s should not exist", rs.Primary.ID)
		}
	}

	return nil
}
