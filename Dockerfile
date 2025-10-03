FROM golang:1.25.1 AS builder
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -ldflags="-s -w" -o /app/spotify-ws .
RUN go build -ldflags="-s -w" -o /app/healthcheck ./healthcheck

FROM gcr.io/distroless/static:nonroot
ARG BUILD_DATE
ARG VCS_REF
LABEL org.opencontainers.image.created=$BUILD_DATE
LABEL org.opencontainers.image.authors="skidoodle"
LABEL org.opencontainers.image.url="https://github.com/skidoodle/spotify-ws"
LABEL org.opencontainers.image.documentation="https://github.com/skidoodle/spotify-ws/blob/main/readme.md"
LABEL org.opencontainers.image.source="https://github.com/skidoodle/spotify-ws"
LABEL org.opencontainers.image.revision=$VCS_REF
LABEL org.opencontainers.image.title="spotify-ws"
LABEL org.opencontainers.image.description="A WebSocket server that relays the current Spotify playing track to connected clients."
LABEL org.opencontainers.image.licenses="AGPL-3.0"
WORKDIR /app
COPY --from=builder --chown=nonroot:nonroot /app/spotify-ws .
COPY --from=builder --chown=nonroot:nonroot /app/healthcheck .
EXPOSE 3000
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s CMD ["/app/healthcheck"]
USER nonroot:nonroot
CMD ["/app/spotify-ws"]