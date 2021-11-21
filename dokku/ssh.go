package dokku

import (
	"errors"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"

	"github.com/melbahja/goph"
)

type SshOutput struct {
	stdout string
	// status code will be 0 if there is no error, otherwise
	// the status code extracted form the error
	status int
	err    error
}

// Run a command using the provided SSH client
//
// strings to be removed from logging can also be provided via `sensitiveStrings`
func run(client *goph.Client, cmd string, sensitiveStrings ...string) SshOutput {

	cmdSafe := cmd
	for _, toReplace := range sensitiveStrings {
		cmdSafe = strings.Replace(cmdSafe, toReplace, "*******", -1)
	}

	log.Printf("[DEBUG] SSH: %s", cmdSafe)

	stdoutRaw, err := client.Run(cmd)

	stdout := string(stdoutRaw)
	for _, toReplace := range sensitiveStrings {
		stdout = strings.Replace(stdout, toReplace, "*******", -1)
	}

	if err != nil {
		status := parseStatusCode(err.Error())
		log.Printf("[DEBUG] SSH: error status %d from %s", status, cmdSafe)
		return SshOutput{
			stdout: stdout,
			status: status,
			err:    errors.New(fmt.Sprintf("Error [%d]: %s", status, stdout)),
		}
	} else {
		return SshOutput{
			stdout: stdout,
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
