FROM golang:1.25.10-alpine3.23 AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /out/server ./cmd/server


FROM alpine:3.22

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /out/server /app/server
COPY --from=builder /src/web /app/web

EXPOSE 8080

ENTRYPOINT ["/app/server"]