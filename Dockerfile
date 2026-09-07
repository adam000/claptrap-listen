####
# Base Go build
####
FROM golang:1.27 AS build
ENV CGO_ENABLED=0

# Warm up the module cache.
# Only copy in go.mod and go.sum to increase Docker cache hit rate.
COPY go.mod go.sum /src/
WORKDIR /src
RUN go mod download

COPY . /src

WORKDIR /src

RUN go build -v -o app

####
# Final build
####
# TODO I should probably pin this to a version of alpine
FROM alpine

RUN apk add --no-cache msmtp openssl ca-certificates

COPY --from=build /src/app /app/

# DOWNLOAD CERTS -------------------------
RUN update-ca-certificates
RUN ln -sf /usr/bin/msmtp /usr/sbin/sendmail
COPY msmtprc /etc

WORKDIR /app

EXPOSE 8080

ENTRYPOINT ["./app", "--rabbitmq"]
