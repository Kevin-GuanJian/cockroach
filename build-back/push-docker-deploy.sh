#!/bin/bash

set -eux

$(dirname $0)/build-docker-deploy.sh

cd $(dirname $0)/../acceptance
if [ -f ./acceptance.test ]; then
  time ./acceptance.test -i dmatrix/cockroach -b /cockroach/cockroach -num 3 \
    -test.v -test.timeout -5m
fi

docker tag dmatrix/cockroach:latest dmatrix/cockroach:${VERSION}

for version in latest ${VERSION}; do
  # Pushing to the registry just fails sometimes, so for the time
  # being just make this a best-effort action.
  docker push dmatrix/cockroach:${version} || true
done
