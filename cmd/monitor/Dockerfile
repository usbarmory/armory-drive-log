# This Dockerfile builds an image with all the tools needed to
# build armory-drive. The entrypoint will run a monitor that
# reproduces the builds from the armory-drive-log.
FROM golang:1.17-alpine AS builder

ARG GOFLAGS=""
ENV GOFLAGS=$GOFLAGS
ENV GO111MODULE=on

# Move to working directory /build
WORKDIR /build

# Copy and download dependency using go mod
COPY go.mod .
COPY go.sum .
RUN go mod download

# Copy the code into the container
COPY . .

# Build the application
RUN go build -o /build/bin/monitor ./cmd/monitor

#
# Set up the final image
#
FROM ubuntu:18.04

RUN apt-get update
RUN apt-get -y install curl unzip wget xxd git

# Tamago bits
RUN apt-get -y install binutils-arm-none-eabi build-essential make u-boot-tools musl-tools
RUN curl -sfL https://github.com/usbarmory/tamago-go/releases/download/tamago-go1.17.1/tamago-go1.17.1.linux-amd64.tar.gz | tar -xzf - -C /
ENV TAMAGO=/usr/local/tamago-go/bin/go

ENV GOPATH /go
ENV PATH $GOPATH/bin:/testbase/protoc/bin:/usr/local/tamago-go/bin:/usr/local/go/bin:$PATH

# Set up the proto compilation stuff
RUN cd /usr && \
    wget "https://github.com/google/protobuf/releases/download/v3.12.4/protoc-3.12.4-linux-x86_64.zip" && \
    unzip "protoc-3.12.4-linux-x86_64.zip"
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.27

COPY --from=builder /build/bin/monitor /bin/monitor

ENTRYPOINT ["/bin/monitor", "--alsologtostderr", "--state_file=/tmp/state"]
