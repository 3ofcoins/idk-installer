package main

import "flag"
import "log"

func main () {
	version := flag.String("version", "stable",
		"Specify version to install (use \"prerelease\" to install latest prerelease)")
	flag.Parse()

	platform, err := detectPlatform()
	if err != nil { log.Fatalln("ERROR detecting platform:", err) }
	log.Println("Detected platform", platform)

	manifest, err := GetManifest()
	if err != nil { log.Fatalln("ERROR getting manifest:", err) }

	pkg, err := manifest.ForPlatform(platform, *version)
	if err != nil { log.Fatalln("ERROR selecting package:", err) }

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
		log.Fatalln("Cannot proceed.")
	}

	log.Println("Selected package", pkg)

	if err := pkg.Download() ; err != nil { log.Fatalln("ERROR downloading package:", err) }
	if err := pkg.Install()  ; err != nil { log.Fatalln("ERROR installing package:",  err) }
}
