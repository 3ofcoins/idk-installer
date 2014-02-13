SOURCES = $(wildcard *.go)
BINARIES = idk-installer-darwin-x86_64 idk-installer-linux-x86_64

idk-installer.sh: $(BINARIES) idk-installer-run.sh
	rm -rf destdir
	mkdir destdir
	cp -a $(BINARIES) idk-installer-run.sh destdir
	./vendor/makeself/makeself.sh destdir idk-installer.sh idk-installer ./idk-installer-run.sh

idk-installer-darwin-x86_64: $(SOURCES)
	env GOOS=darwin GOARCH=amd64 go build -o $@ -ldflags -s

idk-installer-linux-x86_64: $(SOURCES)
	env GOOS=linux GOARCH=amd64 go build -o $@ -ldflags -s

clean:
	rm -rf destdir $(BINARIES) idk-installer
