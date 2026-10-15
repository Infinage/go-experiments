# Build the Go Binary
FROM --platform=$BUILDPLATFORM golang:alpine AS go-builder
WORKDIR /app

ARG TARGETOS
ARG TARGETARCH

COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -ldflags="-s -w" -o /server .

# Minimal Runtime
FROM scratch
WORKDIR /app
COPY --from=go-builder /server /app/server
EXPOSE 8080
CMD ["/app/server"]
