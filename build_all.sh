#/bin/sh

PRG="rest2dhcp"
ARCHS_LINUX="arm64 armv6 armv7 s390x ppc64le amd64 386 riscv64 mips64"
ARCHS_DARWIN="arm64 amd64"
ARCHS_WINDOWS="amd64 arm64 386"

if [ ! -f "$GOPATH/bin/esc" ] ; then
    echo "Install esc"
    go install github.com/mjibson/esc@v0.2.0
fi

DIFF=`git diff --stat`
NOW=`date`
COMMIT=`git rev-parse --short HEAD`

mkdir -p ./build

if [ ! -f "./service/doc.go" ] ; then
    echo "Generate embedded files"
    go generate ./service/server.go
fi

VERSION=$(git describe --tags --always --dirty) 
FLAGS="-X 'main.gitCommit=$COMMIT' -X 'main.buildTime=$NOW' -X 'main.treeState=$STATE' -X 'main.tag=$VERSION' -w -s"
export CGO_ENABLED=0

export GOOS=linux
for arch in $ARCHS_LINUX ; do
    if expr "$arch" : "armv6$" 1>/dev/null; then
        export GOARCH=arm 
        export GOARM=6
    elif expr "$arch" : "armv7$" 1>/dev/null; then
        export GOARCH=arm
        export GOARM=7
    else 
        export GOARCH=$arch
    fi

    echo "Build $PRG-$GOOS-$arch $VERSION" 
    go build -o ./build/$PRG-$GOOS-$arch -ldflags "$FLAGS"
done

export GOOS=darwin
for arch in $ARCHS_DARWIN ; do
    export GOARCH=$arch
    echo "Build $PRG-$GOOS-$arch $VERSION" 
    go build -o ./build/$PRG-$GOOS-$arch -ldflags "$FLAGS"
done

export GOOS=windows
for arch in $ARCHS_WINDOWS ; do
    export GOARCH=$arch
    echo "Build $PRG-$GOOS-$arch $VERSION" 
    go build -o ./build/$PRG-$GOOS-$arch -ldflags "$FLAGS"
done