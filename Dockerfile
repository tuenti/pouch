FROM golang:1.8.3 AS build

ARG version=dev

ENV GOPATH /gopath
ENV SRC /gopath/src/github.com/tuenti/pouch

WORKDIR $SRC
COPY . $SRC
RUN go get -d ./...
RUN go install -ldflags "-X main.version=$version"

FROM ubuntu:17.04
# libsystemd is dynamically loaded by go-systemd
RUN apt-get update && apt-get install -y libsystemd0 && rm -rf /var/lib/apt/lists/*
COPY --from=build /gopath/bin/pouch /usr/bin/pouch
CMD /usr/bin/pouch
