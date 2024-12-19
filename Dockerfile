FROM golang:alpine as builder
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o spotify-ws .

FROM gcr.io/distroless/static:nonroot
WORKDIR /app
COPY --from=builder /app/spotify-ws .
EXPOSE 3000
USER nonroot:nonroot
CMD ["./spotify-ws"]
