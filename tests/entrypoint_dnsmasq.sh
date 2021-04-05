#!/bin/sh

cdr2mask ()
{
   # Number of args to shift, 255..255, first non-255 byte, zeroes
   set -- $(( 5 - ($1 / 8) )) 255 255 255 255 $(( (255 << (8 - ($1 % 8))) & 255 )) 0 0 0
   [ $1 -gt 1 ] && shift $1 || shift
   echo ${1-0}.${2-0}.${3-0}.${4-0}
}

DONE_FILE=/done.txt

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

if [ ! -f "$DONE_FILE" ] ; then
    echo "domain=$DOMAIN" >> /etc/dnsmasq.conf
    echo "dhcp-range=$FIRST,$LAST,$NETMASK,12h" >> /etc/dnsmasq.conf
    echo "dhcp-authoritative" >> /etc/dnsmasq.conf
    touch "$DONE_FILE"
fi

if [ $# -eq 0 ]; then
    dnsmasq -k -8 /dev/stdout -q --log-dhcp
else
    exec "$@"
fi