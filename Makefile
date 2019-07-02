GOPATH := $(shell cd ../../../.. && pwd)
export GOPATH

init-dep:
	@dep init

dep:
	@dep ensure

status-dep:
	@dep status

update-dep:
	@dep ensure -update

test:
	@cd stress && go test -v -race

build-mac:
	@GOOS=darwin GOARCH=amd64 go build -o ./bin/mac/stress
