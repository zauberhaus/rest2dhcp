#!/bin/sh

cdr2mask ()
{
   # Number of args to shift, 255..255, first non-255 byte, zeroes
   set -- $(( 5 - ($1 / 8) )) 255 255 255 255 $(( (255 << (8 - ($1 % 8))) & 255 )) 0 0 0
   [ $1 -gt 1 ] && shift $1 || shift
   echo ${1-0}.${2-0}.${3-0}.${4-0}
}

IP=`ip -o -f inet addr show | awk '/scope global/ {print $4}'`
CDR=`echo $IP | awk -F/ '{ print $2 }'`
SUBNET=`echo $IP | awk -F/ '{ print $1 }' | awk -F. '{print $1 "." $2 "." $3 ".0"}'`
NETMASK=`cdr2mask $CDR`

if [ -z "$DOMAIN" ] ; then
    DOMAIN=`cat /etc/resolv.conf | grep -v "#" |  grep 'search' | awk '{ print $2}'`

    if [ -z "$DOMAIN" ] ; then
        DOMAIN="domain"
    fi
fi

if [ -z "$D" ] ; then
    NAMESERVER=`cat /etc/resolv.conf | grep nameserver | awk '{ print $2}'`
fi

if [ -z "$ROUTER" ] ; then
    ROUTER=`echo $SUBNET | awk -F. '{print $1 "." $2 "." $3 ".1"}'`
fi

if [ -z "$FIRST" ] || [ -z "$LAST" ] ; then
    FIRST=`echo $SUBNET | awk -F. '{print $1 "." $2 "." $3 ".100"}'`
    LAST=`echo $SUBNET | awk -F. '{print $1 "." $2 "." $3 ".200"}'`
fi

if [ ! -f "$CFG_FILE" ] ; then
    echo "authoritative;" > $CFG_FILE && \
    echo "subnet $SUBNET netmask $NETMASK {" >> $CFG_FILE && \
    echo "    option domain-name \"$DOMAIN\";" >> $CFG_FILE && \
    echo "    option domain-name-servers $NAMESERVER;" >> $CFG_FILE && \
    echo "    option routers $ROUTER;" >> $CFG_FILE && \
    echo "    range $FIRST $LAST;" >> $CFG_FILE && \
    echo "}" >> $CFG_FILE 
fi

if [ $# -eq 0 ]; then
    dhcpd -d
else
    exec "$@"
fi