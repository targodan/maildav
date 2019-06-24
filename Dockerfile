# Stage 0
# Build
FROM golang:alpine as builder

RUN apk add --no-cache git

WORKDIR /opt/maildav

COPY ./go.mod ./go.sum ./*.go ./cmd/maildav/*.go ./

RUN go build -v ./...

# Stage 1
# Final image
FROM alpine

COPY --from=builder /opt/maildav/cmd/maildav/maildav /usr/bin

RUN apk --no-cache add ca-certificates

ENV HOME=/opt/maildav
WORKDIR $HOME

VOLUME /etc/maildav

ENTRYPOINT ["maildav", "--config", "/etc/maildav/config.yml"]
