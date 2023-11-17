package sys

import (
	"fmt"
	"os/exec"
)

func Command(cmd string, agrs ...interface{}) (string, error) {
	str := fmt.Sprintf(cmd, agrs...)

	out, err := exec.Command("sh", "-c", str).Output()
	if err != nil {
		return "", fmt.Errorf("%s: %s", str, err.Error())
	}

	return string(out), nil
}
