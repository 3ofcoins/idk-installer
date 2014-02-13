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

var Flags struct {
	prerelease bool
	version string
}

type PlatformInfo struct {
	name string
	version string
	arch string
}

func (pi *PlatformInfo) String() string {
	return strings.Join( []string{pi.name, pi.version, pi.arch} , "-")
}

func (pi *PlatformInfo) MatchMetadata(mdjson []byte) (err error, md map[string]string) {
	if err = json.Unmarshal(mdjson, &md) ; err != nil {
		return
	}
	if pi.name != md["platform"] || pi.arch != md["arch"] {
		return nil, nil
	}
	switch pi.name {
	case "mac_os_x":
		mdpv, err := semver.NewVersion(md["platform_version"])
		if err != nil { return err, nil }
		piv, err := semver.NewVersion(pi.version)
		if err != nil { return err, nil }
		if *mdpv == *piv || mdpv.LessThan(*piv) {
			return nil, md
		} else {
			return nil, nil
		}
	case "arch":
		return nil, md
	default:
		if md["platform_version"] == pi.version {
			return nil, md
		} else {
			return nil, nil
		}
	}
}

func latestVersion() (rv *semver.Version) {
	objs, err := Bucket().List("idk/", "/", "", 1000)
	if err != nil { log.Panic(err) }

	for _, prefix := range(objs.CommonPrefixes) {
		prefix = strings.TrimRight(strings.TrimPrefix(prefix, "idk/"), "/")
		if ver, err := semver.NewVersion(prefix) ; err != nil {
			log.Println("Semver error for", prefix, ":", err)
		} else {
			if (Flags.prerelease || ver.PreRelease == "") && (rv == nil || rv.LessThan(*ver)) {
				rv = ver
			}
		}
	}
	return
}

type IDKPackage struct {
	io.ReadCloser
	bytes int
	metadata map[string]string
}

func S3Metadata(version *semver.Version, pi *PlatformInfo) (error, *IDKPackage) {
	var metadata map[string]string
	packages := make(map[string]s3.Key)
	objs, err := Bucket().List(fmt.Sprintf("idk/%v/", version), "", "", 1000)
	if err != nil { return err, nil }
	for _, key := range(objs.Contents) {
		if strings.HasSuffix(key.Key, ".metadata.json") {
			if md_json, err := Bucket().Get(key.Key) ; err != nil {
				return err, nil
			} else {
				if err, metadata = pi.MatchMetadata(md_json) ; err != nil {
					return err, nil
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
		return errors.New("No package matched"), nil
	}

	reader, err := Bucket().GetReader(packages[metadata["basename"]].Key)
	if err != nil { return err, nil }
	return nil, &IDKPackage{reader, int(packages[metadata["basename"]].Size), metadata}
}

func FromDirectory(dir string, pi *PlatformInfo) (error, *IDKPackage) {
	var metadata map[string]string
	if fifi, err := ioutil.ReadDir(dir) ; err != nil {
		return err, nil
	} else {
		for _, fi := range(fifi) {
			if strings.HasSuffix(fi.Name(), ".metadata.json") {
				if md_json, err := ioutil.ReadFile(path.Join(dir, fi.Name())) ; err != nil {
					return err, nil
				} else {
					if err, metadata = pi.MatchMetadata(md_json) ; err != nil {
						return err, nil
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
		return errors.New("No package matched"), nil
	}

	pkg_path := path.Join(dir, metadata["basename"])
	stat, err := os.Stat(pkg_path)
	if err != nil { return err, nil }

	reader, err := os.Open(pkg_path)
	if err != nil { return err, nil }

	return nil, &IDKPackage{reader, int(stat.Size()), metadata}
}

func (idk *IDKPackage) Install() error {
	defer idk.Close()

	log.Printf("Downloading %v ...\n", idk.metadata["basename"])

	pkg_f, err := os.Create(idk.metadata["basename"])
	if err != nil { panic(err) }
	defer pkg_f.Close()

	dl_sha256 := sha256.New()

	dl_pbar := pb.New(idk.bytes)
	dl_pbar.SetUnits(pb.U_BYTES)
	dl_pbar.Start()
	io.Copy(pkg_f, dl_pbar.NewProxyReader(io.TeeReader(idk, dl_sha256)))

	dl_pbar.Finish()

	if hex.EncodeToString(dl_sha256.Sum(nil)) == idk.metadata["sha256"] {
		log.Println("Checksum matches: sha256", idk.metadata["sha256"])
	} else {
		panic(errors.New("Checksum mismatch"))
	}

	var cmd_words []string
	switch {
	case strings.HasSuffix(idk.metadata["basename"],  ".sh"):
		cmd_words = []string{"/bin/sh", idk.metadata["basename"]}
	case strings.HasSuffix(idk.metadata["basename"], ".deb"):
		cmd_words = []string{"/usr/bin/dpkg", "-i", idk.metadata["basename"]}
	default:
		panic(errors.New("CAN'T HAPPEN"))
	}

	var cmd *exec.Cmd
	if os.Getuid() != 0 {
		cmd = exec.Command("/usr/bin/sudo", cmd_words...)
	} else {
		cmd = exec.Command(cmd_words[0], cmd_words[1:]...)
	}
	cmd.Stdin  = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	log.Println("Running ", cmd.Path, cmd.Args)
	if err := cmd.Run() ; err != nil { panic(err) }

	cmd = exec.Command("/usr/bin/idk", "setup")
	cmd.Stdin  = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	log.Println("Running ", cmd.Path, cmd.Args)
	if err := cmd.Run() ; err != nil { panic(err) }	

	return nil
}

func main () {
	flag.BoolVar(&Flags.prerelease, "pre", false, "Install prerelease version")
	flag.StringVar(&Flags.version, "version", "latest", "Specify version to install")
	flag.Parse()

	pi, err := detectPlatform()
	if err != nil { log.Panic(err) }

	var pkg *IDKPackage
	if pkg_dir := os.Getenv("IDK_PACKAGE_DIR") ; pkg_dir == "" {
		var version *semver.Version
		if Flags.version == "latest" {
			version = latestVersion()
		} else {
			version, err = semver.NewVersion(Flags.version)
			if err != nil { log.Panic(err) }
		}

		err, pkg = S3Metadata(version, pi)
	} else {
		err, pkg = FromDirectory(pkg_dir, pi)
	}
	if err != nil { log.Panic(err) }

	if err := pkg.Install() ; err != nil {
		log.Panic(err)
	}
}
