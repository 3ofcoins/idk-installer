package main

import "flag"
import "encoding/json"
import "io/ioutil"
import "log"
import "os"
import "path"
import "path/filepath"
import "sort"
import "strings"

import "github.com/coreos/go-semver/semver"

const MANIFEST_URL = "https://s3.amazonaws.com/downloads.3ofcoins.net/idk/manifest.json"

type Manifest []*Package
func (p Manifest) Len() int           { return len(p) }
func (p Manifest) Less(i, j int) bool { return p[i].Version().LessThan(*p[j].Version()) }
func (p Manifest) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p Manifest) Sort()              { sort.Sort(sort.Reverse(p)) }

func ManifestFromNetwork(url string) (pkgs Manifest, err error) {
	log.Println("Downloading manifest from", url)

	resp, err := Get(url)
	if err != nil { return nil, Err(err) }
	manifest_json, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil { return nil, Err(err) }
	if err = json.Unmarshal(manifest_json, &pkgs) ; err != nil {
		return nil, Err(err)
	}
	pkgs.Sort()
	return
}

func ManifestFromDir(dir string) (pkgs Manifest, err error) {
	log.Println("Using packages from directory", dir)

	if fifi, err := ioutil.ReadDir(dir) ; err != nil {
		return nil, Err(err)
	} else {
		for _, fi := range(fifi) {
			if strings.HasSuffix(fi.Name(), ".metadata.json") {
				if pkg, err := PackageFromJson(path.Join(dir, fi.Name())) ; err != nil {
					return nil, Err(err)
				} else {
					pkgs = append(pkgs, pkg)					
				}
			}
		}
	}
	pkgs.Sort()
	return
}

func GetManifest() (pkgs Manifest, err error) {
	manifest_loc := flag.Arg(0)
	if manifest_loc == "" {
		manifest_loc = MANIFEST_URL
	}

	if strings.HasPrefix(manifest_loc, "http://") || strings.HasPrefix(manifest_loc, "https://") || strings.HasPrefix(manifest_loc, "file://") {
		return ManifestFromNetwork(manifest_loc)		
	}

	if fi, err := os.Stat(manifest_loc) ; err == nil {
		if fi.IsDir() {
			return ManifestFromDir(manifest_loc)
		} else {
			abspath, err := filepath.Abs(manifest_loc)
			if err != nil { return nil, Err(err) }
			return ManifestFromNetwork(strings.Join([]string{"file://", abspath}, ""))
		}
	} else {
		return nil, Err(err)
	}
}

func (p Manifest) ForPlatform(pi *Platform, version string) (*Package, error) {
	var semVersion *semver.Version
	if version != "stable" && version != "prerelease" {
		sv, err := semver.NewVersion(version)
		if err != nil { return nil, Err(err) }
		semVersion = sv
	}

	for _, pkg := range(p) {
		if semVersion != nil {
			if *pkg.Version() != *semVersion { continue }
		} else {
			if version != "prerelease" && pkg.IsPreRelease() { continue }
		}
	
		if accepts, err := pi.AcceptsPackage(pkg) ; err != nil {
			return nil, Err(err)
		} else {
			if accepts {
				return pkg, nil
			}
		}
	}
	return nil, nil
}
