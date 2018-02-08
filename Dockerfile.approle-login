FROM golang:1.9.4 AS build

ARG version=dev
ARG package=github.com/tuenti/pouch

ENV GOPATH /gopath
ENV SRC $GOPATH/src/$package

WORKDIR $SRC
COPY . $SRC
RUN go install -ldflags "-X main.version=$version" $package/cmd/approle-login

FROM alpine:3.7
RUN apk add --no-cache libc6-compat ca-certificates
COPY --from=build /gopath/bin/approle-login /usr/bin/
CMD /usr/bin/approle-login
