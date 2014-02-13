package main

import "crypto/sha256"
import "encoding/hex"
import "encoding/json"
import "fmt"
import "io"
import "log"
import "os"
import "os/exec"
import "path"
import "sort"
import "strings"

import "launchpad.net/goamz/aws"
import "launchpad.net/goamz/s3"
import "github.com/coreos/go-semver/semver"
import "github.com/cheggaaa/pb"

// IAM idk-installer, read-only access to downloads.3ofcoins.net
const AWS_ACCESS_KEY_ID = "AKIAIYVLKVLRJET2YMZQ"
const AWS_SECRET_ACCESS_KEY = "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"
const S3_BUCKET_NAME = "downloads.3ofcoins.net"

type PlatformInfo struct {
	name string
	version string
	arch string
}

func (pi *PlatformInfo) String() string {
	return strings.Join( []string{pi.name, pi.version, pi.arch} , "-")
}

func (pi *PlatformInfo) MatchMetadata(md map[string]string) bool {
	if pi.name != md["platform"] || pi.arch != md["arch"] {
		return false
	}
	switch pi.name {
	case "mac_os_x":
		mdpv, err := semver.NewVersion(md["platform_version"])
		if err != nil { log.Panic(err) }
		piv, err := semver.NewVersion(pi.version)
		if err != nil { log.Panic(err) }
		return *mdpv == *piv || mdpv.LessThan(*piv)
	case "arch":
		return true
	default:
		return md["platform_version"] == pi.version
	}
}

func main () {
	pi, err := detectPlatform()
	if err != nil {
		log.Panic(err)
	}
	log.Println("Platform:", pi)

	bucket := s3.
		New(aws.Auth{AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY}, aws.Regions["us-east-1"]).
		Bucket(S3_BUCKET_NAME)

	objs, err := bucket.List("idk/", "/", "", 1000)
	if err != nil { log.Panic(err) }

	versions := make(semver.Versions, 0, len(objs.CommonPrefixes))
	for _, prefix := range(objs.CommonPrefixes) {
		prefix = strings.TrimRight(strings.TrimPrefix(prefix, "idk/"), "/")
		if ver, err := semver.NewVersion(prefix) ; err != nil {
			log.Println("Semver error for", prefix, ":", err)
		} else {
			versions = append(versions, ver)
		}
	}
	sort.Sort(sort.Reverse(versions))

	var version *semver.Version
	var metadata map[string]string
	packages := make(map[string]s3.Key)
	for _, ver := range(versions) {
		if ver.PreRelease != "" {
			continue // TODO: switch to allow prerelease
		}
		log.Println("Looking at version", ver)
		objs, err = bucket.List(fmt.Sprintf("idk/%v/", ver), "", "", 1000)
		if err != nil { log.Panic(err) }
		for _, key := range(objs.Contents) {
			if strings.HasSuffix(key.Key, ".metadata.json") {
				if md_json, err := bucket.Get(key.Key) ; err != nil {
					log.Panic(err)
				} else {
					var md map[string]string
					if err := json.Unmarshal(md_json, &md) ; err != nil {
						log.Panic(err)
					}
					if pi.MatchMetadata(md) {
						metadata = md
						break
					}
				}
			} else {
				packages[path.Base(key.Key)] = key
			}
		}
		if len(metadata) > 0 {
			version = ver
			break
		}
	}
	if len(metadata) == 0 {
		log.Panic("No package matched")
	}

	pkg_key := fmt.Sprintf("idk/%v/%v", version, metadata["basename"])
	log.Printf("Downloading %v ...\n", metadata["basename"])

	s3_reader, err := bucket.GetReader(pkg_key)
	if err != nil { log.Panic(err) }
	defer s3_reader.Close()

	pkg_f, err := os.Create(metadata["basename"])
	if err != nil { log.Panic(err) }
	defer pkg_f.Close()

	dl_sha256 := sha256.New()

	dl_pbar := pb.New(int(packages[metadata["basename"]].Size))
	dl_pbar.SetUnits(pb.U_BYTES)
	dl_pbar.Start()
	io.Copy(pkg_f, dl_pbar.NewProxyReader(io.TeeReader(s3_reader, dl_sha256)))

	dl_pbar.Finish()

	if hex.EncodeToString(dl_sha256.Sum(nil)) == metadata["sha256"] {
		log.Println("Checksum matches: sha256", metadata["sha256"])
	} else {
		log.Panic("Checksum mismatch")
	}

	var cmd *exec.Cmd
	switch {
	case strings.HasSuffix(metadata["basename"],  ".sh"):
		cmd = exec.Command("/bin/sh", metadata["basename"])
	case strings.HasSuffix(metadata["basename"], ".deb"):
		cmd = exec.Command("/usr/bin/dpkg", "-i", metadata["basename"])
	default:
		log.Panic("CAN'T HAPPEN")
	}
	if os.Getuid() != 0 {
		cmd.Args = append(cmd.Args, "")
		copy(cmd.Args[1:], cmd.Args)
		cmd.Args[0] = "/usr/bin/sudo"
		cmd.Path = "/usr/bin/sudo"
	}
	log.Println("Running: ", cmd.Path, cmd.Args)
	if err := cmd.Run() ; err != nil { panic(err) }

	cmd = exec.Command("/usr/bin/idk", "setup")
	if err := cmd.Run() ; err != nil { panic(err) }	
}
