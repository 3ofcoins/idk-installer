package main

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
	return platform.semver, Err(err)
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
		platform_sv, err := platform.Semver()
		if err != nil { return false, Err(err) }

		pkg_platform_sv, err := semver.NewVersion(pkg.PlatformVersion)
		if err != nil { return false, Err(err) }

		if *pkg_platform_sv == *platform_sv || pkg_platform_sv.LessThan(*platform_sv) {
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
