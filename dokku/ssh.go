package dokku

import (
	"errors"
	"fmt"
	"log"
	"regexp"
	"strconv"

	"github.com/melbahja/goph"
)

type SshOutput struct {
	stdout string
	// status code will be 0 if there is no error, otherwise
	// the status code extracted form the error
	status int
	err    error
}

//
func run(client *goph.Client, cmd string) SshOutput {
	log.Printf("[DEBUG] SSH: %s", cmd)
	stdout, err := client.Run(cmd)

	if err != nil {
		status := parseStatusCode(err.Error())
		log.Printf("[DEBUG] SSH: error status %d", status)
		return SshOutput{
			stdout: string(stdout),
			status: status,
			err:    errors.New(fmt.Sprintf("Error [%d]: %s", status, string(stdout))),
		}
	} else {
		return SshOutput{
			stdout: string(stdout),
			status: 0,
			err:    nil,
		}
	}
}

// TODO add some debug logging
func parseStatusCode(str string) int {
	re := regexp.MustCompile("^Process exited with status ([0-9]+)$")
	found := re.FindStringSubmatch(str)

	if found == nil {
		return 0
	}

	i, err := strconv.Atoi(found[1])

	if err != nil {
		return 0
	}

	return i
}
