VERSION := 0.0.0
PACKAGE := github.com/tuenti/pouch
ROOT_DIR := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
GOLANG_DOCKER := golang:1.8.1

all:
	docker run -v $(ROOT_DIR):/go/src/$(PACKAGE) -w /go/src/$(PACKAGE) -it --rm $(GOLANG_DOCKER) go build -ldflags "-X main.version=$(VERSION)"

release:
	@if echo $(VERSION) | grep -q "dev$$" ; then echo Set VERSION variable to release; exit 1; fi
	@if git show v$(VERSION) > /dev/null 2>&1; then echo Version $(VERSION) already exists; exit 1; fi
	sed -i "s/^VERSION :=.*/VERSION := $(VERSION)/" Makefile
	git ci Makefile -m "Version $(VERSION)"
	git tag v$(VERSION) -a -m "Version $(VERSION)"
