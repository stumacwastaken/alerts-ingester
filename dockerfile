FROM golang:1.25 AS builder

# Build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .
RUN CGO_ENABLED=0 go build -o alerts-ingester ./cmd/alerts-ingester/main.go

# Copy binary into slim image
FROM alpine
WORKDIR /etc
# RUN ls

RUN apk update && apk add --no-cache sqlite && rm -rf /var/cache/apk/*

COPY --from=builder /src/alerts-ingester .
CMD ["./alerts-ingester"]