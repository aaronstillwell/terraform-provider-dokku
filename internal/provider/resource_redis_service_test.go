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

func TestAccRedisService(t *testing.T) {
	serviceName := fmt.Sprintf("redis-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		Providers:    testAccProviders,
		CheckDestroy: testRedisServiceDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "dokku_redis_service" "test" {
	name = "%s"
}
`, serviceName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRedisServiceExists("dokku_redis_service.test"),
				),
			},
		},
	})
}

func TestAccRedisServiceImage(t *testing.T) {
	serviceName := fmt.Sprintf("redis-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		Providers:    testAccProviders,
		CheckDestroy: testRedisServiceDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "dokku_redis_service" "test" {
	name = "%s"
	image = "circleci/redis"
	image_version = "6.2.5"
}
`, serviceName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRedisServiceExists("dokku_redis_service.test"),
					testAccCheckRedisImageAndVersion("dokku_redis_service.test", "circleci/redis", "6.2.5"),
				),
			},
		},
	})
}

func TestAccRedisExposedOn(t *testing.T) {
	serviceName := fmt.Sprintf("redis-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		Providers:    testAccProviders,
		CheckDestroy: testRedisServiceDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "dokku_redis_service" "test" {
	name = "%s"
}
`, serviceName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRedisServiceExists("dokku_redis_service.test"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "dokku_redis_service" "test" {
	name = "%s"
	expose_on = "0.0.0.0:8585"
}
`, serviceName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRedisServiceExists("dokku_redis_service.test"),
					testAccCheckRedisExposed("dokku_redis_service.test", true, "0.0.0.0:8585"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "dokku_redis_service" "test" {
	name = "%s"
}
`, serviceName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRedisServiceExists("dokku_redis_service.test"),
					testAccCheckRedisExposed("dokku_redis_service.test", false, ""),
				),
			},
		},
	})
}

func testAccCheckRedisServiceExists(n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]

		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("Service ID not present")
		}

		sshClient := testAccProvider.Meta().(*goph.Client)

		service := NewDokkuRedisService(rs.Primary.ID)
		err := dokkuRedisRead(service, sshClient)

		if err != nil {
			return fmt.Errorf("Error reading redis resource %s", rs.Primary.ID)
		}

		if service.Id == "" {
			return fmt.Errorf("Service %s was not created", rs.Primary.ID)
		}

		return nil
	}
}

func testAccCheckRedisImageAndVersion(n string, image string, version string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]

		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("Service ID not present")
		}

		sshClient := testAccProvider.Meta().(*goph.Client)

		service := NewDokkuRedisService(rs.Primary.ID)
		err := dokkuRedisRead(service, sshClient)

		if err != nil {
			return fmt.Errorf("Error reading redis resource %s", rs.Primary.ID)
		}

		if service.Image != image {
			return fmt.Errorf("Image expected to be %s, got %s", image, service.Image)
		}

		if service.ImageVersion != version {
			return fmt.Errorf("Image version expected to be %s, got %s", version, service.ImageVersion)
		}

		return nil
	}
}

func testAccCheckRedisExposed(n string, isExposed bool, host string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]

		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("Service ID not present")
		}

		sshClient := testAccProvider.Meta().(*goph.Client)

		service := NewDokkuRedisService(rs.Primary.ID)
		err := dokkuRedisRead(service, sshClient)

		if err != nil {
			return fmt.Errorf("Error reading redis resource %s", rs.Primary.ID)
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

func testRedisServiceDestroy(s *terraform.State) error {
	sshClient := testAccProvider.Meta().(*goph.Client)

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "dokku_redis_service" {
			continue
		}

		service := NewDokkuRedisService(rs.Primary.ID)
		err := dokkuRedisRead(service, sshClient)

		if err != nil {
			return fmt.Errorf("Dokku redis service %s could not be read: %v", rs.Primary.ID, err)
		}

		if service.Id != "" {
			return fmt.Errorf("Dokku redis service %s should not exist", rs.Primary.ID)
		}
	}

	return nil
}
