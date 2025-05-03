package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/melbahja/goph"
)

func TestAccMongodbService(t *testing.T) {
	serviceName := fmt.Sprintf("mongo-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		Providers:    testAccProviders,
		CheckDestroy: testMongodbServiceDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "dokku_mongodb_service" "test" {
	name = "%s"
}
`, serviceName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckMongodbServiceExists("dokku_mongodb_service.test"),
					testAccCheckMongodbExposed("dokku_mongodb_service.test", false, ""),
				),
			},
		},
	})
}

func TestAccMongodbServiceImage(t *testing.T) {
	serviceName := fmt.Sprintf("mongo-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		Providers:    testAccProviders,
		CheckDestroy: testMongodbServiceDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "dokku_mongodb_service" "test" {
	name = "%s"
	image = "mongo"
	image_version = "6.0"
}
`, serviceName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckMongodbServiceExists("dokku_mongodb_service.test"),
					testAccCheckMongodbServiceImageAndVersion("dokku_mongodb_service.test", "mongo", "6.0"),
				),
			},
		},
	})
}

func TestAccMongodbUpdate(t *testing.T) {
	serviceName := fmt.Sprintf("mongo-%s", acctest.RandString(10))
	newServiceName := fmt.Sprintf("mongo-renamed-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		Providers:    testAccProviders,
		CheckDestroy: testMongodbServiceDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "dokku_mongodb_service" "test" {
	name = "%s"
}
`, serviceName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckMongodbServiceExists("dokku_mongodb_service.test"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "dokku_mongodb_service" "test" {
	name = "%s"
	image = "mongo"
	image_version = "6.0"
}
`, newServiceName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckMongodbServiceExists("dokku_mongodb_service.test"),
					testAccCheckMongodbServiceName("dokku_mongodb_service.test", newServiceName),
					testAccCheckMongodbServiceImageAndVersion("dokku_mongodb_service.test", "mongo", "6.0"),
				),
			},
		},
	})
}

func TestAccReadStoppedMongodbService(t *testing.T) {
	serviceName := fmt.Sprintf("mongo-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		Providers:    testAccProviders,
		CheckDestroy: testMongodbServiceDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "dokku_mongodb_service" "test" {
	name = "%s"
}
`, serviceName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckMongodbServiceExists("dokku_mongodb_service.test"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "dokku_mongodb_service" "test" {
	name    = "%s"
	stopped = true
}
`, serviceName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckMongodbServiceExists("dokku_mongodb_service.test"),
					testAccCheckMongodbServiceStopped("dokku_mongodb_service.test", true),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "dokku_mongodb_service" "test" {
	name    = "%s"
	stopped = false
}
`, serviceName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckMongodbServiceExists("dokku_mongodb_service.test"),
					testAccCheckMongodbServiceStopped("dokku_mongodb_service.test", false),
				),
			},
		},
	})
}

func TestAccMongodbExposedOn(t *testing.T) {
	serviceName := fmt.Sprintf("mongo-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		Providers:    testAccProviders,
		CheckDestroy: testMongodbServiceDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "dokku_mongodb_service" "test" {
	name = "%s"
}
`, serviceName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckMongodbServiceExists("dokku_mongodb_service.test"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "dokku_mongodb_service" "test" {
	name = "%s"
	expose_on = "0.0.0.0:27017"
}
`, serviceName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckMongodbServiceExists("dokku_mongodb_service.test"),
					testAccCheckMongodbExposed("dokku_mongodb_service.test", true, "0.0.0.0:27017"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "dokku_mongodb_service" "test" {
	name = "%s"
}
`, serviceName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckMongodbServiceExists("dokku_mongodb_service.test"),
					testAccCheckMongodbExposed("dokku_mongodb_service.test", false, ""),
				),
			},
		},
	})
}

func TestAccMongodbExposedOnCreate(t *testing.T) {
	serviceName := fmt.Sprintf("mongo-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		Providers:    testAccProviders,
		CheckDestroy: testMongodbServiceDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "dokku_mongodb_service" "test" {
	name = "%s"
	expose_on = "0.0.0.0:27017"
}
`, serviceName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckMongodbServiceExists("dokku_mongodb_service.test"),
					testAccCheckMongodbExposed("dokku_mongodb_service.test", true, "0.0.0.0:27017"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "dokku_mongodb_service" "test" {
	name = "%s"
}
`, serviceName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckMongodbServiceExists("dokku_mongodb_service.test"),
					testAccCheckMongodbExposed("dokku_mongodb_service.test", false, ""),
				),
			},
		},
	})
}

func testAccCheckMongodbServiceExists(n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]

		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("Service ID not present")
		}

		sshClient := testAccProvider.Meta().(*goph.Client)

		service := NewDokkuMongodbService(rs.Primary.ID)
		err := dokkuMongodbRead(service, sshClient)

		if err != nil {
			return fmt.Errorf("Error reading mongodb resource %s", rs.Primary.ID)
		}

		if service.Id == "" {
			return fmt.Errorf("Service %s was not created", rs.Primary.ID)
		}

		return nil
	}
}

func testAccCheckMongodbServiceName(n string, name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]

		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("Service ID not present")
		}

		sshClient := testAccProvider.Meta().(*goph.Client)

		service := NewDokkuMongodbService(rs.Primary.ID)
		err := dokkuMongodbRead(service, sshClient)

		if err != nil {
			return fmt.Errorf("Error reading mongodb resource %s", rs.Primary.ID)
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

func testAccCheckMongodbServiceImageAndVersion(n string, image string, version string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]

		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("Service ID not present")
		}

		sshClient := testAccProvider.Meta().(*goph.Client)

		service := NewDokkuMongodbService(rs.Primary.ID)
		err := dokkuMongodbRead(service, sshClient)

		if err != nil {
			return fmt.Errorf("Error reading mongodb resource %s", rs.Primary.ID)
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

func testAccCheckMongodbServiceStopped(n string, isStopped bool) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]

		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("Service ID not present")
		}

		sshClient := testAccProvider.Meta().(*goph.Client)

		service := NewDokkuMongodbService(rs.Primary.ID)
		err := dokkuMongodbRead(service, sshClient)

		if err != nil {
			return fmt.Errorf("Error reading mongodb resource %s", rs.Primary.ID)
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

func testAccCheckMongodbExposed(n string, isExposed bool, host string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]

		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("Service ID not present")
		}

		sshClient := testAccProvider.Meta().(*goph.Client)

		service := NewDokkuMongodbService(rs.Primary.ID)
		err := dokkuMongodbRead(service, sshClient)

		if err != nil {
			return fmt.Errorf("Error reading mongodb resource %s", rs.Primary.ID)
		}

		if isExposed {
			if service.Exposed != host {
				return fmt.Errorf("mongodb was not exposed as expected, returned %s", service.Exposed)
			}
		} else {
			if service.Exposed != "" {
				return fmt.Errorf("Service was exposed unexpectedly, returned %s", service.Exposed)
			}
		}

		return nil
	}
}

func testMongodbServiceDestroy(s *terraform.State) error {
	sshClient := testAccProvider.Meta().(*goph.Client)

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "dokku_mongodb_service" {
			continue
		}

		service := NewDokkuMongodbService(rs.Primary.ID)
		err := dokkuMongodbRead(service, sshClient)

		if err != nil {
			return fmt.Errorf("Dokku mongodb service %s could not be read: %v", rs.Primary.ID, err)
		}

		if service.Id != "" {
			return fmt.Errorf("Dokku mongodb service %s should not exist", rs.Primary.ID)
		}
	}

	return nil
}
