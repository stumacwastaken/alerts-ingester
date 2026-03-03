FROM golang:1.25 AS builder

# Build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .
RUN CGO_ENABLED=0 go build -o alerts-server .

EXPOSE 9200
# Copy binary into slim image
FROM alpine
WORKDIR /etc
COPY --from=builder /src/alerts-server .
CMD ["/etc/alerts-server"]
