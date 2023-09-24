package tun

import (
	"fmt"
	"os/exec"
)

func command(cmd string, agrs ...interface{}) (string, error) {
	str := fmt.Sprintf(cmd, agrs...)

	out, err := exec.Command("sh", "-c", str).Output()
	if err != nil {
		return "", fmt.Errorf("%s: %e", cmd, err)
	}

	return string(out), nil
}
