package main

import "crypto/sha256"
import "encoding/hex"
import "encoding/json"
import "fmt"
import "io"
import "io/ioutil"
import "log"
import "os"
import "os/exec"
import "path"
import "path/filepath"
import "strings"

import "github.com/cheggaaa/pb"
import "github.com/coreos/go-semver/semver"

type Package struct {
	Arch string `json:"arch"`
	Basename string `json:"basename"`
	Bytes int `json:"bytes"`
	Md5 string `json:"md5"`
	Platform string `json:"platform"`
	PlatformVersion string `json:"platform_version"`
	Sha256 string `json:"sha256"`
	URL string `json:"url"`
	VersionString string `json:"version"`
	semVersion *semver.Version `json:"-"`
}

func PackageFromJson(json_path string) (*Package, error) {
	var pkg Package
	if metadata_json, err := ioutil.ReadFile(json_path) ; err != nil {
		return nil, Err(err)
	} else {
		if err := json.Unmarshal(metadata_json, &pkg) ; err != nil {
			return nil, Err(err)
		}
		pkg_path, err := filepath.Abs(path.Join(path.Dir(json_path), pkg.Basename))
		if err != nil { return nil, Err(err) }
		pkg.URL = strings.Join([]string{"file://", pkg_path}, "")
		if fi, err := os.Stat(pkg_path) ; err != nil {
			return nil, Err(err)
		} else {
			pkg.Bytes = int(fi.Size())
		}
		return &pkg, nil
	}
}

func (pkg *Package) Version() *semver.Version {
	if pkg.semVersion == nil {
		if sv, err := semver.NewVersion(pkg.VersionString) ; err != nil {
			panic(err)
		} else {
			pkg.semVersion = sv
		}
	}
	return pkg.semVersion
}

func (pkg *Package) String() string {
	return fmt.Sprintf("<%v (%v) %v-%v-%v>",
		pkg.Basename, pkg.Version(),
		pkg.Platform, pkg.PlatformVersion, pkg.Arch)
}

func (pkg *Package) IsPreRelease() bool {
	return pkg.Version().Metadata != ""
}

func (pkg *Package) Download() error {
	log.Println("Downloading package from", pkg.URL)

	resp, err := Get(pkg.URL)
	if err != nil { return Err(err) }
	defer resp.Body.Close()

	// Create output file to write to
	pkg_f, err := os.Create(pkg.Basename)
	if err != nil { return Err(err) }
	defer pkg_f.Close()

	// Calculate checksum as we go
	dl_sha256 := sha256.New()

	// Display progress bar as we go
	dl_pbar := pb.New(pkg.Bytes)
	dl_pbar.SetUnits(pb.U_BYTES)

	dl_pbar.Start()
	io.Copy(pkg_f, dl_pbar.NewProxyReader(io.TeeReader(resp.Body, dl_sha256)))
	dl_pbar.Finish()

	// Verify download
	if dl_sha256 := hex.EncodeToString(dl_sha256.Sum(nil)) ; dl_sha256 == pkg.Sha256 {
		log.Println("Package checksum correct: SHA256", pkg.Sha256)
	} else {
		return NewErrf("Package checksum mismatch: expected SHA256 %v, got %v",
			pkg.Sha256, dl_sha256)
	}

	return nil
}

func runCommand(words ...string) error {
	cmd := exec.Command(words[0], words[1:]...)
	cmd.Stdin  = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	log.Println("Running", words)
	if err := cmd.Run() ; err != nil {
		return Errf(err, "%v", words)
	}
	return nil
}

func (pkg *Package) Install() error {
	// Compose installation command
	var install_command []string
	if os.Getuid() != 0 {
		tmpdir := os.Getenv("TMPDIR")
		if tmpdir == "" {
			// Fallback. Normally we have tmpdir set by the run script.
			tmpdir = "/tmp"
		}
		install_command = append(install_command, "/usr/bin/sudo", "/usr/bin/env", fmt.Sprintf("TMPDIR=%v", tmpdir))
	}

	switch {
	case strings.HasSuffix(pkg.Basename,  ".sh"):
		install_command = append(install_command,
			"/bin/sh", pkg.Basename)
	case strings.HasSuffix(pkg.Basename, ".deb"):
		install_command = append(install_command,
			"/usr/bin/dpkg", "-i", pkg.Basename)
	default:
		return NewErrf("Unrecognized package type %v", pkg.Basename)
	}

	// Install PKG
	if err := runCommand(install_command...) ; err != nil { return Err(err) }

	// Setup IDK
	if err := runCommand("/opt/idk/bin/idk", "setup") ; err != nil { return Err(err) }

	return nil
}
