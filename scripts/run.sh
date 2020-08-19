#!/bin/sh

ROUTE=`ip route get 1.1.2.1 | head -n1`
SERVER=`echo $ROUTE |  awk '{ print $3}'`
RELAY=`echo $ROUTE |  awk '{ print $7}'`

IMAGE="rest2dhcp:latest"

if test ! -f "`pwd`/rest2dhcp"; then
    echo "Build appliaction"
    ./build.sh
fi

SERVER=$SERVER RELAY=$RELAY ./rest2dhcp $@
