#!/bin/sh
exec ./idk-installer-`uname -s | tr A-Z a-z`-`uname -m` "${@}"
