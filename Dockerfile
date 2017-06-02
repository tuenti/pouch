FROM golang:1.8.3 AS build

ARG version=dev
ARG package=github.com/tuenti/pouch

ENV GOPATH /gopath
ENV SRC $GOPATH/src/$package

WORKDIR $SRC
COPY . $SRC
RUN go test . ./cmd/...
RUN go install -ldflags "-X main.version=$version" $package/cmd/...

FROM ubuntu:17.04
# libsystemd is dynamically loaded by go-systemd
RUN apt-get update && apt-get install -y libsystemd0 openssh-client && rm -rf /var/lib/apt/lists/*
COPY --from=build /gopath/bin/* /usr/bin/
CMD /usr/bin/pouch
