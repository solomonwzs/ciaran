# vim: noet:

BASE = ${PWD}
GOPATH := ${BASE}:${GOPATH}
MAIN = "src/main.go"
BINPATH = "bin"


all:
	@go build -o $(BINPATH)/main.goc $(MAIN)

run:all
	@$(BINPATH)/main.goc
