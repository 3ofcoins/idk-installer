Infrastructure Development Kit Installer
========================================

This project is a universal installer for the
[Infrastructure Development Kit](https://github.com/3ofcoins/idk/). The
installer will run on any system supported by IDK (which is Arch and
Ubuntu LTS Linux on ia64 and MacOS for now). It will select, install,
and set up newest (or selected) version of IDK with a single
command.

The most up-to-date version can always be downloaded from the address
https://s3.amazonaws.com/downloads.3ofcoins.net/idk-installer.sh

How do I use it?
----------------

Download the script. Run it. IDK should be installed.

To see the archiver's options, run `idk-installer.sh --help`.

To see the installer's options, run `idk-installer.sh -- -help`.


How does it work?
-----------------

The `idk-installer.sh` script is a compressed
[Makeself](http://megastep.org/makeself/) archive. It contains two
statically compiled installer binaries, written in Go: one for Linux,
and one for Mac OS X (both for 64-bit Intel architecture). When the
script runs, the archive unpacks to a temporary directory and a setup
shell script is started. This script just looks at current operating
system & architecture, and runs appropriate installer binary.

The binary inspects system in more detail, looks for the most recent
version (or a version specified on command line) of IDK for the
system, downloads the package from S3, installs it, and runs `idk
setup` to configure the system.

Does it have to be so complex?
------------------------------

Possibly not. On the other hand, as of now, this approach seems to
provide the best user experience overall:

 - There is one installer, user doesn't have to choose between many
   similar looking Omnibus packages.
 - Using a separate installer makes it possible to install and setup
   IDK in a single pass; adding setup to package's post-install script
   breaks because chef-solo cookbooks may want to install packages
   themselves.
 - Go binaries provide decent balance between portability (static
   binaries without external dependencies, so there's one binary per
   architecture, and only two architectures are supported) and
   development convenience (writing lowest-common-denominator shell
   scripts is painful). With Go it is possible to implement some logic
   in the installer: listing S3 bucket, finding most recent package
   version, download itself, client system inspection, integrity
   checks, etc.
 - Makeself provides a safe, clean temporary directory to run in -
   it makes it easy to avoid leaving garbage around.
 
 The downside is that the installer itself is quite big (3MB
 compressed), and it includes two separate binaries. But remember,
 we're speaking about installing a 100+MB piece of software that
 unpacks to 300MB+ directory; I believe that extra convenience and
 flexibility is worth it. Also, the installer is independent from IDK
 itself, so it can be cached by IDK and reused for subsequent
 upgrades. IDK can upgrade the cached installer (which should happen
 rarely), and the installer can upgrade IDK - no package will need to
 overwrite itself.
