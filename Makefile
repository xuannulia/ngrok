.PHONY: default server client admin deps fmt clean all release-client release-server release-admin release-all contributors
export GOPATH:=$(shell pwd)
export GO111MODULE:=off

BUILDTAGS=debug
default: all

deps:
	go list -tags '$(BUILDTAGS)' ngrok/... >/dev/null

server: deps
	go install -tags '$(BUILDTAGS)' ngrok/main/ngrokd

fmt:
	go fmt ngrok/...

client: deps
	go install -tags '$(BUILDTAGS)' ngrok/main/ngrok

admin: deps
	go install -tags '$(BUILDTAGS)' ngrok/main/ngrok-admin

release-client: BUILDTAGS=release
release-client: client

release-server: BUILDTAGS=release
release-server: server

release-admin: BUILDTAGS=release
release-admin: admin

release-all: fmt release-client release-server release-admin

all: fmt client server admin

clean:
	go clean -i -r ngrok/...
	rm -rf bin/ pkg/

contributors:
	echo "Contributors to ngrok, both large and small:\n" > CONTRIBUTORS
	git log --raw | grep "^Author: " | sort | uniq | cut -d ' ' -f2- | sed 's/^/- /' | cut -d '<' -f1 >> CONTRIBUTORS
