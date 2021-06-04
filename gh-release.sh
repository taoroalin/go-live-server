#!/bin/bash

GH_NAME="taoroalin/go-live-server"

LAST_COMMIT_HASH=$(git ls-remote git://github.com/$GH_NAME.git | \
   grep refs/heads/master | cut -f 1)

echo $LAST_COMMIT_HASH

ARCH=amd64
for OS in windows linux darwin; do
  env GOOS=$OS GOARCH=$ARCH go build -o dist/go-live-server-$OS-$ARCH-wink
done

# create release
curl \
  -X POST \
  -H "Accept: application/vnd.github.v3+json" \
  https://api.github.com/repos/taoroalin/go-live-server/releases \
  -d "{\"tag_name\":\"$LAST_COMMIT_HASH\",\"name\":\"$LAST_COMMIT_HASH\",\"prerelease\":true}"

# add files to release
curl \
  -X PATCH \
  -H "Accept: application/vnd.github.v3+json" \
  https://api.github.com/repos/taoroalin/go-live-server/releases/assets/42 \
  -d '{"name":"name"}'