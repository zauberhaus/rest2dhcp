#!/bin/sh

if test -z $1 ; then
    VERSION="latest"
else
    VERSION="$1"    
fi

TAG="docker.io/zauberhaus/rest2dhcp:$VERSION"

INSPECT=`docker buildx imagetools inspect $TAG --raw`

IMAGES=`echo $INSPECT | jq -r 'select(.manifests != null) | .manifests[] | .digest + "/" + .platform.architecture + "/" + .platform.variant'`

rm -rf build
mkdir -p build && cd build

if test -z "$IMAGES" ;then
    id=`docker create $TAG`
    docker cp $id:/rest2dhcp - > out.tar
    docker rm -v $id > /dev/null
    FILES="rest2dhcp-`arch`"
    tar xf out.tar && \
    rm out.tar && \
    mv rest2dhcp $FILES
else
    for IMAGE in $IMAGES ; do
        HASH=`echo $IMAGE | awk -F/ '{ print $1}' `
        ARCH=`echo $IMAGE | awk -F/ '{ print $2}' `
        VARIANT=`echo $IMAGE | awk -F/ '{ print $3}' `

        NAME="$TAG@$HASH"

        id=`docker create $NAME`
        docker cp $id:/rest2dhcp - > out.tar
        docker rm -v $id > /dev/null
        docker rmi $NAME

        tar xf out.tar && \
        rm out.tar
        if test -z "$VARIANT"; then 
            mv rest2dhcp rest2dhcp-$ARCH
            FILES="$FILES rest2dhcp-$ARCH"
        else
            mv rest2dhcp rest2dhcp-$ARCH-$VARIANT
            FILES="$FILES rest2dhcp-$ARCH-$VARIANT"
        fi    
    done
fi

echo $FILES

