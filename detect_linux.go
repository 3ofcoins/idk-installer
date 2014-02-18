// +build linux

package main

import "io/ioutil"
import "os"
import "os/exec"
import "strings"

func detectPlatform() (*Platform, error) {
	var rv Platform

	if out, err := exec.Command("uname", "-m").Output() ; err != nil {
		return nil, Err(err)
	} else {
		rv.arch = strings.TrimSpace(string(out))
	}
	
	if f, err := os.Open("/etc/lsb-release") ; err == nil {
		defer f.Close()
		content, err := ioutil.ReadAll(f)
		if err != nil { return nil, Err(err) }
		for _, ln := range(strings.Split(string(content), "\n")) {
			splut := strings.SplitN(ln, "=", 2)
			switch splut[0] {
			case "DISTRIB_ID": rv.name = strings.ToLower(splut[1])
			case "DISTRIB_RELEASE": rv.version = splut[1]
			}
		}
		return &rv, nil
	} else if !os.IsNotExist(err) {
		return nil, Err(err)
	}

	if f, err := os.Open("/etc/debian_version") ; err == nil {
		defer f.Close()
		rv.name = "debian"
		if content, err := ioutil.ReadAll(f) ; err != nil {
			return nil, Err(err)
		} else {
			rv.version = strings.TrimSpace(string(content))
		}
		return &rv, nil
	} else if !os.IsNotExist(err) {
		return nil, Err(err)
	}

	if _, err := os.Stat("/etc/arch-release") ; err == nil {
		rv.name = "arch"
		rv.version = "*"
		return &rv, nil
	} else if !os.IsNotExist(err) {
		return nil, Err(err)
	}
	
	return nil, NewErrf("Undetected platform")
}
