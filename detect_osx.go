// +build darwin

package main

import "errors"
import "os"
import "os/exec"
import "strings"

func detectPlatform() (*PlatformInfo, error) {
	var rv PlatformInfo

	if out, err := exec.Command("uname", "-m").Output() ; err != nil {
		return nil, err
	} else {
		rv.arch = strings.TrimSpace(string(out))
	}
	
	if _, err := os.Stat("/usr/bin/sw_vers") ; err != nil {
		return nil, err
	}

	rv.name = "mac_os_x"
	if out, err := exec.Command("/usr/bin/sw_vers").Output() ; err != nil {
		return nil, err
	} else {
		for _, line := range(strings.Split(string(out), "\n")) {
			splut := strings.SplitN(line, ":\t", 2)
			if splut[0] == "ProductVersion" {
				rv.version = splut[1]
				return &rv, nil
			}
		}
	}
	
	return nil, errors.New("CAN'T HAPPEN")
}
