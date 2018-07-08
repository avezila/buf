#!/bin/bash
ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$ROOT"
set -e


_REPO=github.com/avezila/buf
_SERVICE=buf
_REPOROOT=$PWD/../..
_GOPATH=${GOPATH:-/home/$USER/go}


time docker run           \
  -u $(id -u):$(id -g)          \
  -v $_GOPATH:/go               \
  -v $_REPOROOT:/go/src/$_REPO  \
  -w /go/src/$_REPO/$_SERVICE   \
  avezila/go /bin/bash -c "
    set -e
    GOGC=off go build -i -ldflags '-w' -o ./app .
"

docker build -t buf/$_SERVICE -f ./Dockerfile ..

rm ../app
