#!/bin/sh

VERSION=3.96

case `arch` in
    x86_64)
        ARCH=amd64
        ;;
    aarch64)
        ARCH=arm64
        break
        ;;
    ppc64le)
        ARCH=powerpc64le
        break
        ;;
    armv7l)
        ARCH=arm
        break
        ;;
    *)
        echo "Unknown platform: `arch`"
        exit 1
        ;;
esac

wget https://github.com/upx/upx/releases/download/v${VERSION}/upx-${VERSION}-${ARCH}_linux.tar.xz
tar xJf upx-${VERSION}-${ARCH}_linux.tar.xz
cp upx-${VERSION}-${ARCH}_linux/upx /usr/local/bin/upx
rm -rf upx-${VERSION}-${ARCH}_linux*