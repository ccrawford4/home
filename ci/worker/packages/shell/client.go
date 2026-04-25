package shell

import "os/exec"

func ExecuteCommand(name string, args ...string) (string, error) {
	output, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return string(output), err
	}
	return string(output), nil
}
