#!/usr/bin/env sh

go build . || exit 1
docker build -t git.d464.sh/diogo464/belverde-fire -f Containerfile . || exit 1

if [ "$PUSH" = "1" ]; then
	docker push git.d464.sh/diogo464/belverde-fire
fi

