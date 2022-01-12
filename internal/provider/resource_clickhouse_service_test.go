package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/melbahja/goph"
)

func TestAccClickhouseService(t *testing.T) {
	serviceName := fmt.Sprintf("clickhouse-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		Providers:    testAccProviders,
		CheckDestroy: testClickhouseServiceDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "dokku_clickhouse_service" "test" {
	name = "%s"
}
`, serviceName),
				Check: testClickhouseServiceExists("dokku_clickhouse_service.test"),
			},
			{
				Config: fmt.Sprintf(`
resource "dokku_clickhouse_service" "test" {
	name = "%s"
	stopped = true
}
`, serviceName),
				Check: resource.ComposeTestCheckFunc(
					testClickhouseServiceExists("dokku_clickhouse_service.test"),
					testClickhouseServiceIsStopped("dokku_clickhouse_service.test", true),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "dokku_clickhouse_service" "test" {
	name = "%s"
	stopped = false
}
`, serviceName),
				Check: resource.ComposeTestCheckFunc(
					testClickhouseServiceExists("dokku_clickhouse_service.test"),
					testClickhouseServiceIsStopped("dokku_clickhouse_service.test", false),
				),
			},
		},
	})
}

func TestAccClickhouseServiceCreateStopped(t *testing.T) {
	serviceName := fmt.Sprintf("clickhouse-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		Providers:    testAccProviders,
		CheckDestroy: testClickhouseServiceDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "dokku_clickhouse_service" "test" {
	name = "%s"
	stopped = true
}
`, serviceName),
				Check: resource.ComposeTestCheckFunc(
					testClickhouseServiceExists("dokku_clickhouse_service.test"),
					testClickhouseServiceIsStopped("dokku_clickhouse_service.test", true),
				),
			},
		},
	})
}

func testClickhouseServiceExists(n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]

		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("Service ID not present")
		}

		sshClient := testAccProvider.Meta().(*goph.Client)

		service, err := getServiceInfo("clickhouse", rs.Primary.ID, sshClient)

		if err != nil {
			return fmt.Errorf("Error reading clickhouse resource %s", rs.Primary.ID)
		}

		if service == nil {
			return fmt.Errorf("Service %s was not created", rs.Primary.ID)
		}

		return nil
	}
}

func testClickhouseServiceIsStopped(n string, isStopped bool) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]

		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("Service ID not present")
		}

		sshClient := testAccProvider.Meta().(*goph.Client)

		service, err := getServiceInfo("clickhouse", rs.Primary.ID, sshClient)

		if err != nil {
			return fmt.Errorf("Error reading clickhouse resource %s", rs.Primary.ID)
		}

		if service == nil {
			return fmt.Errorf("Service %s was not created", rs.Primary.ID)
		}

		serviceIsStopped := service["status"] == "exited"
		if serviceIsStopped != isStopped {
			return fmt.Errorf("Service %s returned stopped = %v, expected %v - status was %s", rs.Primary.ID, serviceIsStopped, isStopped, service["status"])
		}

		return nil
	}
}

func testClickhouseServiceDestroy(s *terraform.State) error {
	sshClient := testAccProvider.Meta().(*goph.Client)

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "dokku_clickhouse_service" {
			continue
		}

		service, err := getServiceInfo("clickhouse", rs.Primary.ID, sshClient)

		if err != nil {
			return fmt.Errorf("Dokku clickhouse service %s could not be read: %v", rs.Primary.ID, err)
		}

		if service != nil {
			return fmt.Errorf("Dokku clickhouse service %s should not exist", rs.Primary.ID)
		}
	}

	return nil
}
