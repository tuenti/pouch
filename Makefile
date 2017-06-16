VERSION := 0.2.1
DOCKER_REPOSITORY := tuenti/pouch
DOCKER_TAG := $(DOCKER_REPOSITORY):$(VERSION)
DOCKER_IMAGE_FILE := $(subst /,-,$(DOCKER_REPOSITORY))-$(VERSION).docker

all: test bins docker aci

docker:
	docker build --build-arg version=$(VERSION) -t $(DOCKER_TAG) .

$(DOCKER_IMAGE_FILE): docker
	docker save $(DOCKER_TAG) -o $(DOCKER_IMAGE_FILE)

aci: $(DOCKER_IMAGE_FILE)
	docker2aci $(DOCKER_IMAGE_FILE)

test:
	go test -tags testutils . ./pkg/... ./cmd/...

bins: bin/pouch bin/pouchctl bin/terraform-provisioner-vault-secret-id bin/approle-login

bin/%: cmd/%/
	go build -ldflags "-X main.version=$(VERSION)" -o $@ -i ./$<

install:
	go install -a -ldflags "-X main.version=$(VERSION)" ./cmd/...

clean:
	rm -f pouch pouchctl *.docker *.aci bin/*

release:
	@if echo $(VERSION) | grep -q "dev$$" ; then echo Set VERSION variable to release; exit 1; fi
	@if git show v$(VERSION) > /dev/null 2>&1; then echo Version $(VERSION) already exists; exit 1; fi
	sed -i "s/^VERSION :=.*/VERSION := $(VERSION)/" Makefile
	git ci Makefile -m "Version $(VERSION)"
	git tag v$(VERSION) -a -m "Version $(VERSION)"
