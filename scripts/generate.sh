#1/bin/sh

TAGS=`git describe --tags 2> /dev/null`
sed "s/version: \"1.0.0\"/version: \"$TAGS\"/g" api/swagger.yaml > doc/swagger.yaml