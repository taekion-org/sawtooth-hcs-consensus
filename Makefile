include Makefile.inc

.PHONY: engine util docker

all: engine util

engine:
	${GO_BINARY} build -o build/sawtooth-hcs-consensus

util:
	${GO_BINARY} build -o build/genesis_hcs util/genesis_hcs.go util/common.go

docker:
	docker buildx build --push --platform linux/amd64,linux/arm64 --tag taekion/sawtooth-hcs-consensus -f docker/Dockerfile ..

docker-testing:
	docker buildx build --push --platform linux/amd64,linux/arm64 --tag taekion/sawtooth-hcs-consensus:testing -f docker/Dockerfile ..

clean:
	rm -rfv build/*
