include Makefile.inc

.PHONY: engine util

all: engine util

engine:
	${GO_BINARY} build -o build/sawtooth-hcs-consensus

util:
	${GO_BINARY} build -o build/genesis_hcs util/genesis_hcs.go util/common.go

clean:
	rm -rfv build/*
