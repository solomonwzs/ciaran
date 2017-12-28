# vim: noet:

BASE 			= ${PWD}
GOPATH 			:= ${BASE}:${GOPATH}
TEMPLATE 		= "main.template"
TEST_TEMPLATE 	= "test.template"
MAIN 			= "src/main.go"
BINPATH 		= "bin"

APPS 		= $(wildcard src/application/*/main.go)
FLAGS 		= "-ldflags \"-s -w\""

define target =
$(patsubst src/application/%/main.goc,%,$@)
endef

all:pre ${APPS:%.go=%.goc}
	@echo "all ok"

pre:
	@[ -d bin ] || mkdir bin

reversetunnel_test:reversetunnel
	@./test/reversetunnel_test.sh

%.goc:
	@echo -ne "\033[01;32m[Build]\033[0m $(target) ... "
	@sed "s/%/$(target)/" $(TEMPLATE) > $(MAIN)
	@go build -o $(BINPATH)/$(target).goc $(MAIN)
	@echo -e "ok"

%:
	@echo -ne "\033[01;32m[Build]\033[0m $@ ... "
	@sed "s/%/$@/" $(TEMPLATE) > $(MAIN)
	@go build -o $(BINPATH)/$@.goc $(MAIN)
	@echo -e "ok"
