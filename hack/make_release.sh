#!/usr/bin/env bash

set -e
set -x

[[ $# -lt 1 ]] && echo "version to be released is required" && exit 2
[[ ! "$1" =~ ^v[0-9].[0-9].[0-9]$ ]] && echo "version must be in the form of v0.0.0" && exit 2

VERSION=$1
TAG=$1
BRANCH=${VERSION:1:${#VERSION}-2}0

if [[ $(git branch -l ${BRANCH}) == "" ]]; then
  echo "create branch ${BRANCH}"
  git checkout -b ${BRANCH}
else
  git checkout ${BRANCH}
fi

# Debug out
echo $1 $VERSION $TAG $BRANCH
git branch --show-current

echo "build for linux amd64"
make linux
pushd _output
tar cf kubectl-dev.linux-amd64.tar kubectl-dev
pixz -p 4 kubectl-dev.linux-amd64.tar
popd

echo "build for darwin amd64"
make mac
pushd _output
tar cf kubectl-dev.darwin-amd64.tar kubectl-dev
pixz -p 4 kubectl-dev.darwin-amd64.tar
popd

git push origin $BRANCH

set +x
set +e
