# Stage 0
# Build
FROM golang:stretch as builder

RUN apt-get install git

WORKDIR /opt/maildav

COPY ./go.mod ./go.sum ./
RUN go mod download

COPY ./*.go ./
COPY ./cmd/maildav/*.go ./cmd/maildav/

WORKDIR /opt/maildav/cmd/maildav
RUN CGO_ENABLED=0 go build -v

# Stage 1
# Final image
FROM alpine

RUN apk --no-cache add ca-certificates

COPY --from=builder /opt/maildav/cmd/maildav/maildav /usr/bin

ENV HOME=/opt/maildav
WORKDIR $HOME

VOLUME /etc/maildav

CMD ["/usr/bin/maildav", "--config", "/etc/maildav/config.yml"]
