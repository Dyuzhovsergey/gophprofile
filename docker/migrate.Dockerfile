# syntax=docker/dockerfile:1.7

FROM golang:1.25.10-alpine3.23 AS builder

WORKDIR /src

COPY go.mod go.sum ./

RUN --mount=type=cache,id=gophprofile-gomod,target=/go/pkg/mod \
    go mod download

COPY . .

RUN --mount=type=cache,id=gophprofile-gomod,target=/go/pkg/mod \
    --mount=type=cache,id=gophprofile-gobuild,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/migrate ./cmd/migrate


FROM alpine:3.22

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /out/migrate /app/migrate
COPY --from=builder /src/migrations /app/migrations

ENTRYPOINT ["/app/migrate"]