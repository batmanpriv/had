//go:build windows

package core

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func getTermWidth() (int, error) {
	cmd := exec.Command("cmd", "/C", "mode", "con")
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(line), "columns") {
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				n, err := strconv.Atoi(strings.TrimSpace(parts[1]))
				if err == nil && n > 0 {
					return n, nil
				}
			}
		}
	}
	return 0, fmt.Errorf("could not determine terminal width")
}