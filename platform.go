package main

import "runtime/debug"
import "strings"

import "github.com/coreos/go-semver/semver"

type Platform struct {
	name string
	version string
	arch string
	semver *semver.Version
}

func (platform *Platform) Semver() (*semver.Version, error) {
	var err error
	if platform.semver == nil {
		platform.semver, err = semver.NewVersion(platform.version)
	}
	return platform.semver, err
}

func (platform *Platform) String() string {
	return strings.Join( []string{platform.name, platform.version, platform.arch} , "-")
}

func (platform *Platform) AcceptsPackage(pkg *Package) (bool, error) {
	if platform.name != pkg.Platform || platform.arch != pkg.Arch {
		return false, nil
	}
	switch platform.name {
	case "mac_os_x":
		psv, err := platform.Semver()
		if err != nil { debug.PrintStack() ; return false, err }
		if *pkg.Version() == *psv || pkg.Version().LessThan(*psv) {
			return true, nil
		} else {
			return false, nil
		}
	case "arch":
		return true, nil
	default:
		if pkg.PlatformVersion == platform.version {
			return true, nil
		} else {
			return false, nil
		}
	}
}
