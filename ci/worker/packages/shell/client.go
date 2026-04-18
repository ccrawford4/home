package shell

import "os/exec"

func ExecuteCommand(cmd string) (string, error) {
	output, err := exec.Command(cmd).Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}
