# spotify-ws

The genesis of this project stems from the inherent limitation posed by Spotify, which currently lacks direct support for accessing their API through WebSockets. In light of this restriction, developers are often compelled to resort to interval polling as the primary means of obtaining real-time data.

It's important to note that this project does not purport to resolve the challenge posed by interval polling. Instead, it takes an innovative approach by introducing a Socket.IO server. This server acts as an intermediary, relaying the current playing track to connected clients. Notably, it addresses the issue of redundancy by omitting duplicate transmissions, ensuring that clients only receive updates when the currently playing song changes.

## Running Locally

### With Docker
```
git clone https://github.com/skidoodle/spotify-ws
cd spotify-ws
docker build -t spotify-ws:main .
docker run -p 3000:3000 spotify-ws
```

### Without Docker
```
git clone https://github.com/skidoodle/spotify-ws
cd spotify-ws
npm install
npm run dev
```

## Deploying

### Docker Compose
```
version: '3.9'
services:
    spotify-ws:
        image: ghcr.io/skidoodle/spotify-ws:main
        restart: always
        ports:
            - '3000:3000'
        environment:
            - ENDPOINT=
            #- HOST=
            #- PORT=
```

## License

[AGPL-3.0](https://github.com/skidoodle/spotify-ws/blob/main/license)
