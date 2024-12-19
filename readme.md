# spotify-ws

The genesis of this project stems from the inherent limitation posed by Spotify, which currently lacks direct support for accessing their API through WebSockets. In light of this restriction, developers are often compelled to resort to interval polling as the primary means of obtaining real-time data.

It's important to note that this project does not purport to resolve the challenge posed by interval polling. Instead, it takes an innovative approach by introducing a WebSocket server. This server acts as an intermediary, relaying the current playing track to connected clients. Notably, it addresses the issue of redundancy by omitting duplicate transmissions, ensuring that clients only receive updates when the currently playing song changes.

## Running Locally

### With Docker

```sh
git clone https://github.com/skidoodle/spotify-ws
cd spotify-ws
docker build -t spotify-ws:main .
docker run -p 3000:3000 spotify-ws:main
```

### Without Docker

```sh
git clone https://github.com/skidoodle/spotify-ws
cd spotify-ws
go get
go run main.go
```

## Deploying

### Docker compose

```yaml
services:
    spotify-ws:
        container_name: spotify-ws
        image: 'ghcr.io/skidoodle/spotify-ws:main'
        restart: unless-stopped
        environment:
            - REFRESH_TOKEN=
            - CLIENT_SECRET=
            - CLIENT_ID=
            #- LOG_LEVEL=debug
        ports:
            - '3000:3000'
```

### Docker run

```sh
docker run \
  -d \
  --name=spotify-ws \
  --restart=unless-stopped \
  -p 3000:3000 \
  -e CLIENT_ID= \
  -e CLIENT_SECRET= \
  -e REFRESH_TOKEN= \
  #-e LOG_LEVEL=DEBUG \
  ghcr.io/skidoodle/spotify-ws:main
```

## License

[AGPL-3.0](https://github.com/skidoodle/spotify-ws/blob/main/license)
