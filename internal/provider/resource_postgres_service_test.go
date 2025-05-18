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

func TestAccPostgresService(t *testing.T) {
	serviceName := fmt.Sprintf("pg-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		Providers:    testAccProviders,
		CheckDestroy: testPgServiceDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "dokku_postgres_service" "test" {
	name = "%s"
}
`, serviceName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPgServiceExists("dokku_postgres_service.test"),
					testAccCheckPgExposed("dokku_postgres_service.test", false, ""),
				),
			},
		},
	})
}

func TestAccPostgresServiceImage(t *testing.T) {
	serviceName := fmt.Sprintf("pg-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		Providers:    testAccProviders,
		CheckDestroy: testPgServiceDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "dokku_postgres_service" "test" {
	name = "%s"
	image = "cimg/postgres"
	image_version = "16.4-postgis"
}
`, serviceName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPgServiceExists("dokku_postgres_service.test"),
					testAccCheckPgServiceImageAndVersion("dokku_postgres_service.test", "cimg/postgres", "16.4-postgis"),
				),
			},
		},
	})
}

func TestAccPostgresUpdate(t *testing.T) {
	serviceName := fmt.Sprintf("pg-%s", acctest.RandString(10))
	newServiceName := fmt.Sprintf("pg-renamed-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		Providers:    testAccProviders,
		CheckDestroy: testPgServiceDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "dokku_postgres_service" "test" {
	name = "%s"
}
`, serviceName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPgServiceExists("dokku_postgres_service.test"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "dokku_postgres_service" "test" {
	name = "%s"
	image = "cimg/postgres"
	image_version = "16.4-postgis"
}
`, newServiceName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPgServiceExists("dokku_postgres_service.test"),
					testAccCheckPgServiceName("dokku_postgres_service.test", newServiceName),
					testAccCheckPgServiceImageAndVersion("dokku_postgres_service.test", "cimg/postgres", "16.4-postgis"),
				),
			},
		},
	})
}

// Previously had a problem reading stopped services
// https://github.com/aaronstillwell/terraform-provider-dokku/issues/17
func TestAccReadStoppedPostgresService(t *testing.T) {
	serviceName := fmt.Sprintf("pg-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		Providers:    testAccProviders,
		CheckDestroy: testPgServiceDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "dokku_postgres_service" "test" {
	name = "%s"
}
`, serviceName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPgServiceExists("dokku_postgres_service.test"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "dokku_postgres_service" "test" {
	name    = "%s"
	stopped = true
}
`, serviceName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPgServiceExists("dokku_postgres_service.test"),
					testAccCheckPgServiceStopped("dokku_postgres_service.test", true),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "dokku_postgres_service" "test" {
	name    = "%s"
	stopped = false
}
`, serviceName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPgServiceExists("dokku_postgres_service.test"),
					testAccCheckPgServiceStopped("dokku_postgres_service.test", false),
				),
			},
		},
	})
}

func TestAccPostgresExposedOn(t *testing.T) {
	serviceName := fmt.Sprintf("pg-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		Providers:    testAccProviders,
		CheckDestroy: testPgServiceDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "dokku_postgres_service" "test" {
	name = "%s"
}
`, serviceName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPgServiceExists("dokku_postgres_service.test"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "dokku_postgres_service" "test" {
	name = "%s"
	expose_on = "0.0.0.0:8585"
}
`, serviceName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPgServiceExists("dokku_postgres_service.test"),
					testAccCheckPgExposed("dokku_postgres_service.test", true, "0.0.0.0:8585"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "dokku_postgres_service" "test" {
	name = "%s"
}
`, serviceName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPgServiceExists("dokku_postgres_service.test"),
					testAccCheckPgExposed("dokku_postgres_service.test", false, ""),
				),
			},
		},
	})
}

func TestAccPostgresExposedOnCreate(t *testing.T) {
	serviceName := fmt.Sprintf("pg-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		Providers:    testAccProviders,
		CheckDestroy: testPgServiceDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "dokku_postgres_service" "test" {
	name = "%s"
	expose_on = "0.0.0.0:8585"
}
`, serviceName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPgServiceExists("dokku_postgres_service.test"),
					testAccCheckPgExposed("dokku_postgres_service.test", true, "0.0.0.0:8585"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "dokku_postgres_service" "test" {
	name = "%s"
}
`, serviceName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPgServiceExists("dokku_postgres_service.test"),
					testAccCheckPgExposed("dokku_postgres_service.test", false, ""),
				),
			},
		},
	})
}

func testAccCheckPgServiceExists(n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]

		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("Service ID not present")
		}

		sshClient := testAccProvider.Meta().(*goph.Client)

		service := NewDokkuPostgresService(rs.Primary.ID)
		err := dokkuPgRead(service, sshClient)

		if err != nil {
			return fmt.Errorf("Error reading pg resource %s", rs.Primary.ID)
		}

		if service.Id == "" {
			return fmt.Errorf("Service %s was not created", rs.Primary.ID)
		}

		return nil
	}
}

func testAccCheckPgServiceName(n string, name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]

		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("Service ID not present")
		}

		sshClient := testAccProvider.Meta().(*goph.Client)

		service := NewDokkuPostgresService(rs.Primary.ID)
		err := dokkuPgRead(service, sshClient)

		if err != nil {
			return fmt.Errorf("Error reading pg resource %s", rs.Primary.ID)
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

func testAccCheckPgServiceImageAndVersion(n string, image string, version string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]

		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("Service ID not present")
		}

		sshClient := testAccProvider.Meta().(*goph.Client)

		service := NewDokkuPostgresService(rs.Primary.ID)
		err := dokkuPgRead(service, sshClient)

		if err != nil {
			return fmt.Errorf("Error reading pg resource %s", rs.Primary.ID)
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

func testAccCheckPgServiceStopped(n string, isStopped bool) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]

		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("Service ID not present")
		}

		sshClient := testAccProvider.Meta().(*goph.Client)

		service := NewDokkuPostgresService(rs.Primary.ID)
		err := dokkuPgRead(service, sshClient)

		if err != nil {
			return fmt.Errorf("Error reading pg resource %s", rs.Primary.ID)
		}

		if isStopped && !service.Stopped {
			return fmt.Errorf("Service %s expected to be stopped, but it seems to be running", n)
		}

		if !isStopped && service.Stopped {
			return fmt.Errorf("Service %s expected to be running, but it seems to be stopped", n)
		}

		return nil
	}
}

func testAccCheckPgExposed(n string, isExposed bool, host string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]

		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("Service ID not present")
		}

		sshClient := testAccProvider.Meta().(*goph.Client)

		service := NewDokkuPostgresService(rs.Primary.ID)
		err := dokkuPgRead(service, sshClient)

		if err != nil {
			return fmt.Errorf("Error reading pg resource %s", rs.Primary.ID)
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

func testPgServiceDestroy(s *terraform.State) error {
	sshClient := testAccProvider.Meta().(*goph.Client)

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "dokku_postgres_service" {
			continue
		}

		service := NewDokkuPostgresService(rs.Primary.ID)
		err := dokkuPgRead(service, sshClient)

		if err != nil {
			return fmt.Errorf("Dokku postgres service %s could not be read: %v", rs.Primary.ID, err)
		}

		if service.Id != "" {
			return fmt.Errorf("Dokku postgres service %s should not exist", rs.Primary.ID)
		}
	}

	return nil
}
