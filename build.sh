#!/bin/sh

export eTAG="latest-dev"
echo $1
if [ $1 ] ; then
  eTAG=$1
fi

echo Building ngduchai/rts-cli:$eTAG

docker build --build-arg http_proxy=$http_proxy --build-arg https_proxy=$https_proxy -t ngduchai/rts-cli:$eTAG . && \
 docker create --name rts-cli ngduchai/rts-cli:$eTAG && \
 docker cp rts-cli:/usr/bin/rts-cli . && \
 docker rm -f rts-cli

