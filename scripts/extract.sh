#!/bin/sh

if test -z $1 ; then
    TAG="docker.io/zauberhaus/rest2dhcp:latest"
else
    TAG="docker.io/zauberhaus/rest2dhcp:$1"    
fi

if docker buildx > /dev/null 2>&1 ; then
    if echo "$TAG" | grep "^docker.io/" > /dev/null ; then
        INSPECT=`docker buildx imagetools inspect $TAG --raw`
        IMAGES=`echo $INSPECT | jq -r 'select(.manifests != null) | .manifests[] | .digest + "/" + .platform.architecture + "/" + .platform.variant'`
    fi
fi

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

        echo "$ARCH -> $NAME"

        id=`docker create --platform $ARCH $NAME`
        docker cp $id:/rest2dhcp - > out.tar
        docker rm -v $id > /dev/null
        docker rmi $NAME

        tar xf out.tar && \
        rm out.tar
        if test -z "$VARIANT"; then 
            echo "Copy rest2dhcp to rest2dhcp-$ARCH"
            mv rest2dhcp rest2dhcp-$ARCH
            FILES="$FILES rest2dhcp-$ARCH"
        else
            echo "Copy rest2dhcp to rest2dhcp-$ARCH-$VARIANT"
            mv rest2dhcp rest2dhcp-$ARCH-$VARIANT
            FILES="$FILES rest2dhcp-$ARCH-$VARIANT"
        fi    
    done
fi

echo $FILES

