FROM golang:1.26-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /bin/server ./cmd/server

FROM alpine:3.21

RUN apk add --no-cache ca-certificates

COPY --from=builder /bin/server /bin/server
COPY migrations /app/migrations
COPY templates /app/templates
COPY static /app/static

WORKDIR /app

EXPOSE 8080

ENTRYPOINT ["/bin/server"]
