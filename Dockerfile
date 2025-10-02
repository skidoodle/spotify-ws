FROM golang:1.25.1 AS builder
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -ldflags="-s -w" -o spotify-ws .

FROM gcr.io/distroless/static:nonroot
WORKDIR /app
COPY --from=builder --chown=nonroot:nonroot /app/spotify-ws .

EXPOSE 3000
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s CMD ["/spotify-ws", "-health"]
USER nonroot:nonroot
CMD ["/spotify-ws"]