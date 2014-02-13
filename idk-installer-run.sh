#!/bin/sh
osarch=`uname -s | tr A-Z a-z`-`uname -m`
export TMPDIR=`pwd`
if [ -x ./idk-installer-$osarch ] ; then
    exec ./idk-installer-$osarch "${@}"
else
    echo -n "No installer found for $osarch. Available installers:" >&2
    echo idk-installer-*-* | sed s/idk-installer-//g >&2
    exit 1
fi
