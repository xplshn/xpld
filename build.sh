#!/bin/sh -eux

for pair in $(go tool dist list); do
	GOOS=${pair%/*}
	GOARCH=${pair#*/}

	export GOOS GOARCH
	out="./$(basename "$PWD")_${GOARCH}_${GOOS}"

	go build -o "$out" && {
	    strip -sx "$out" || :
	    cp "$out" "$out.upx"
	    upx "$out.upx" || rm "$out.upx"
	}
done
