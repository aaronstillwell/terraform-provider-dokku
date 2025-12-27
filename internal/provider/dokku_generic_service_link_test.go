package provider

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/melbahja/goph"
)

func testAccServiceLinkExists(serviceCmd string, serviceName string, appName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		sshClient := testAccProvider.Meta().(*goph.Client)

		out := run(sshClient, fmt.Sprintf("%s:linked %s %s", serviceCmd, serviceName, appName))

		if out.err != nil {
			return fmt.Errorf("%s service %s not linked to app %s - %v", serviceCmd, serviceName, appName, out.err)
		}
		return nil
	}
}

func testAccServiceLinkNotExists(serviceCmd string, serviceName string, appName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		sshClient := testAccProvider.Meta().(*goph.Client)

		out := run(sshClient, fmt.Sprintf("%s:linked %s %s", serviceCmd, serviceName, appName))

		if out.err == nil {
			return fmt.Errorf("%s service %s still linked to app %s - %v", serviceCmd, serviceName, appName, out.err)
		}
		return nil
	}
}
