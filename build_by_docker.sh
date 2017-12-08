#!/usr/bin/env bash
docker run --rm -v "$PWD":/usr/src/myapp -w /usr/src/myapp daptin_build_env bash /usr/src/myapp/scripts/build.sh
