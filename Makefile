VERSION := 0.1.0
DOCKER_REPOSITORY := tuenti/pouch
DOCKER_TAG := $(DOCKER_REPOSITORY):$(VERSION)
DOCKER_IMAGE_FILE := $(subst /,-,$(DOCKER_REPOSITORY))-$(VERSION).docker

all:
	docker build --build-arg version=$(VERSION) -t $(DOCKER_TAG) .

$(DOCKER_IMAGE_FILE): all
	docker save $(DOCKER_TAG) -o $(DOCKER_IMAGE_FILE)

aci: $(DOCKER_IMAGE_FILE)
	docker2aci $(DOCKER_IMAGE_FILE)

clean:
	rm -f pouch pouchctl *.docker *.aci

release:
	@if echo $(VERSION) | grep -q "dev$$" ; then echo Set VERSION variable to release; exit 1; fi
	@if git show v$(VERSION) > /dev/null 2>&1; then echo Version $(VERSION) already exists; exit 1; fi
	sed -i "s/^VERSION :=.*/VERSION := $(VERSION)/" Makefile
	git ci Makefile -m "Version $(VERSION)"
	git tag v$(VERSION) -a -m "Version $(VERSION)"
