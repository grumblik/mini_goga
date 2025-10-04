FROM golang:1.25 AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w -X main.version=${VERSION}" -o /out/mini_goga .

FROM alpine:3.20
RUN adduser -D --uid 1000 -H -s /sbin/nologin app && apk add --no-cache ca-certificates
COPY --from=builder /out/mini_goga /usr/local/bin/mini_goga

USER app
EXPOSE 9100
ENTRYPOINT ["mini_goga"]
