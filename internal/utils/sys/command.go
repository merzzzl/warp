package sys

import (
	"fmt"
	"os/exec"
)

// Command execute command and return output and error.
func Command(cmd string, agrs ...any) (string, error) {
	str := fmt.Sprintf(cmd, agrs...)

	out, err := exec.Command("sh", "-c", str).Output()
	if err != nil {
		return "", fmt.Errorf("%s: %w", str, err)
	}

	return string(out), nil
}
