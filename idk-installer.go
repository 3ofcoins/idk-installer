package main

import "flag"
import "log"

func main () {
	version := flag.String("version", "stable",
		"Specify version to install (use \"prerelease\" to install latest prerelease)")
	flag.Parse()

	platform, err := detectPlatform()
	if err != nil { log.Fatal(err) }

	manifest, err := GetManifest()
	if err != nil { log.Fatal(err) }

	pkg, err := manifest.ForPlatform(platform, *version)
	if err != nil { log.Fatal(err) }

	if pkg == nil {
		log.Println("Could not select a package for", platform, "at", version)
		if len(manifest) > 0 {
			log.Println("Manifest contains packages:")
			for _, pkg := range(manifest) {
				log.Println("-", pkg)
			}
		} else {
			log.Println("Manifest was empty.")
		}
		log.Fatal("Cannot proceed.")
	}

	log.Println("Selected package", pkg)

	if err := pkg.Download() ; err != nil { log.Fatal(err) }
	if err := pkg.Install()  ; err != nil { log.Fatal(err) }
}
