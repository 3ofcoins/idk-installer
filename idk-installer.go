package main

import "crypto/sha256"
import "encoding/hex"
import "encoding/json"
import "errors"
import "flag"
import "fmt"
import "io"
import "io/ioutil"
import "log"
import "os"
import "os/exec"
import "path"
import "strings"

import "launchpad.net/goamz/aws"
import "launchpad.net/goamz/s3"
import "github.com/coreos/go-semver/semver"
import "github.com/cheggaaa/pb"

// Command line switches
var Flags struct {
	prerelease bool
	version string
}

// IAM idk-installer, read-only access to downloads.3ofcoins.net
const AWS_ACCESS_KEY_ID = "AKIAIYVLKVLRJET2YMZQ"
const AWS_SECRET_ACCESS_KEY = "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"
const S3_BUCKET_NAME = "downloads.3ofcoins.net"

var _bucket *s3.Bucket
func Bucket() *s3.Bucket {
	if _bucket == nil {
		_bucket = s3.
			New(aws.Auth{AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY}, aws.Regions["us-east-1"]).
			Bucket(S3_BUCKET_NAME)
	}
	return _bucket
}

func idkSemVersion() (rv *semver.Version, err error) {
	if Flags.version == "latest" {
		log.Println("Finding latest IDK version ...")
		objs, err := Bucket().List("idk/", "/", "", 1000)
		if err != nil { return nil, err }

		for _, prefix := range(objs.CommonPrefixes) {
			prefix = strings.TrimRight(strings.TrimPrefix(prefix, "idk/"), "/")
			if ver, err := semver.NewVersion(prefix) ; err != nil {
				log.Println("WARNING: Semver error for", prefix, ":", err)
			} else {
				if (Flags.prerelease || ver.PreRelease == "") && (rv == nil || rv.LessThan(*ver)) {
					rv = ver
				}
			}
		}
		log.Println("Found latest version", rv)
	} else {
		rv, err = semver.NewVersion(Flags.version)
		log.Println("Using IDK version", rv)
	}
	return rv, nil
}

type PlatformInfo struct {
	name string
	version string
	arch string
	semver *semver.Version
}

func (pi *PlatformInfo) Semver() (*semver.Version, error) {
	var err error
	if pi.semver == nil {
		pi.semver, err = semver.NewVersion(pi.version)
	}
	return pi.semver, err
}

func (pi *PlatformInfo) String() string {
	return strings.Join( []string{pi.name, pi.version, pi.arch} , "-")
}

func (pi *PlatformInfo) MatchMetadata(mdjson []byte) (md map[string]string, err error) {
	if err = json.Unmarshal(mdjson, &md) ; err != nil {
		return
	}
	if pi.name != md["platform"] || pi.arch != md["arch"] {
		return nil, nil
	}
	switch pi.name {
	case "mac_os_x":
		mdpv, err := semver.NewVersion(md["platform_version"])
		if err != nil { return nil, err }
		piv, err := pi.Semver()
		if err != nil { return nil, err }
		if *mdpv == *piv || mdpv.LessThan(*piv) {
			return md, nil
		} else {
			return nil, nil
		}
	case "arch":
		return md, nil
	default:
		if md["platform_version"] == pi.version {
			return md, nil
		} else {
			return nil, nil
		}
	}
}

type IDKPackage struct {
	io.ReadCloser
	bytes int
	metadata map[string]string
}

func (pi *PlatformInfo) s3Package(version *semver.Version) (*IDKPackage, error) {
	var metadata map[string]string
	packages := make(map[string]s3.Key)
	objs, err := Bucket().List(fmt.Sprintf("idk/%v/", version), "", "", 1000)
	if err != nil { return nil, err }
	for _, key := range(objs.Contents) {
		if strings.HasSuffix(key.Key, ".metadata.json") {
			if md_json, err := Bucket().Get(key.Key) ; err != nil {
				return nil, err
			} else {
				if metadata, err = pi.MatchMetadata(md_json) ; err != nil {
					return nil, err
				} else {
					if len(metadata) > 0 {
						break
					}
				}
			}
		} else {
			packages[path.Base(key.Key)] = key
		}
	}

	if len(metadata) == 0 {
		return nil, fmt.Errorf("Could not find package version %v for platform %v", version, pi)
	}

	reader, err := Bucket().GetReader(packages[metadata["basename"]].Key)
	if err != nil { return nil, err }
	return &IDKPackage{reader, int(packages[metadata["basename"]].Size), metadata}, nil
}

func (pi *PlatformInfo) dirPackage(dir string) (*IDKPackage, error) {
	log.Println("Using packages from directory", dir)

	var metadata map[string]string
	if fifi, err := ioutil.ReadDir(dir) ; err != nil {
		return nil, err
	} else {
		for _, fi := range(fifi) {
			if strings.HasSuffix(fi.Name(), ".metadata.json") {
				if md_json, err := ioutil.ReadFile(path.Join(dir, fi.Name())) ; err != nil {
					return nil, err
				} else {
					if metadata, err = pi.MatchMetadata(md_json) ; err != nil {
						return nil, err
					} else {
						if len(metadata) > 0 {
							break
						}
					}
				}
			}
		}
	}

	if len(metadata) == 0 {
		return nil, errors.New("No package matched")
	}

	pkg_path := path.Join(dir, metadata["basename"])
	stat, err := os.Stat(pkg_path)
	if err != nil { return nil, err }

	reader, err := os.Open(pkg_path)
	if err != nil { return nil, err }

	return &IDKPackage{reader, int(stat.Size()), metadata}, nil
}

func (pi *PlatformInfo) Package() (*IDKPackage, error) {
	if strings.HasPrefix(Flags.version, "/") || strings.HasPrefix(Flags.version, "./") {
		return pi.dirPackage(Flags.version)
	} else {
		version, err := idkSemVersion()
		if err != nil { return nil, err }
		return pi.s3Package(version)
	}
}

func runCommand(words ...string) error {
	cmd := exec.Command(words[0], words[1:]...)
	cmd.Stdin  = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	log.Println("Running", words)
	if err := cmd.Run() ; err != nil {
		return fmt.Errorf("Failed to run %v: %v", words, err)
	}
	return nil
}

func (idk *IDKPackage) Install() error {
	defer idk.Close()

	log.Printf("Downloading %v ...\n", idk.metadata["basename"])

	// Create output file to write to
	pkg_f, err := os.Create(idk.metadata["basename"])
	if err != nil { return err }
	defer pkg_f.Close()

	// Calculate checksum as we go
	dl_sha256 := sha256.New()

	// Display progress bar as we go
	dl_pbar := pb.New(idk.bytes)
	dl_pbar.SetUnits(pb.U_BYTES)

	dl_pbar.Start()
	io.Copy(pkg_f, dl_pbar.NewProxyReader(io.TeeReader(idk, dl_sha256)))
	dl_pbar.Finish()

	// Verify download
	if dl_sha256 := hex.EncodeToString(dl_sha256.Sum(nil)) ; dl_sha256 == idk.metadata["sha256"] {
		log.Println("Package checksum correct: sha256", idk.metadata["sha256"])
	} else {
		return fmt.Errorf("Package checksum mismatch: expected %v, got %v",
			idk.metadata["sha256"], dl_sha256)
	}

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
	case strings.HasSuffix(idk.metadata["basename"],  ".sh"):
		install_command = append(install_command,
			"/bin/sh", idk.metadata["basename"])
	case strings.HasSuffix(idk.metadata["basename"], ".deb"):
		install_command = append(install_command,
			"/usr/bin/dpkg", "-i", idk.metadata["basename"])
	default:
		return fmt.Errorf("Unrecognized package type %v", idk.metadata["basename"])
	}

	// Install IDK
	if err := runCommand(install_command...) ; err != nil { return err }

	// Setup IDK
	if err := runCommand("/opt/idk/bin/idk", "setup") ; err != nil { return err }

	return nil
}

func main () {
	flag.BoolVar(&Flags.prerelease, "pre", false, "Install prerelease version")
	flag.StringVar(&Flags.version, "version", "latest", "Specify version to install")
	flag.Parse()

	pi, err := detectPlatform()
	if err != nil { log.Fatal(err) }

	pkg, err := pi.Package()
	if err != nil { log.Fatal(err) }

	if err := pkg.Install() ; err != nil {
		log.Fatal(err)
	}
}
