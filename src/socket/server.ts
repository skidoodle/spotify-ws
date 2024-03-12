import * as http from 'http'
import { Server as SocketIOServer, type Socket } from 'socket.io'
import 'dotenv/config'
import { SpotifyService } from '../spotify/service'

function getEnvVar(key: string): string {
  const value = process.env[key]
  if (!value) {
    throw new Error(`Missing environment variable: ${key}`)
  }
  return value
}

const { CLIENT_ID, CLIENT_SECRET, REFRESH_TOKEN } = {
  CLIENT_ID: getEnvVar('CLIENT_ID'),
  CLIENT_SECRET: getEnvVar('CLIENT_SECRET'),
  REFRESH_TOKEN: getEnvVar('REFRESH_TOKEN'),
}

const server = http.createServer()
const io = new SocketIOServer(server, {
  cors: {
    origin: '*',
  },
})

let previousData: string

const spotify = new SpotifyService(CLIENT_ID, CLIENT_SECRET, REFRESH_TOKEN)

const sendNowPlayingData = async () => {
  try {
    const song = await spotify.getCurrentSong()

    if (song && song.is_playing) {
      const data = JSON.stringify(song)

      if (!isEqual(data, previousData)) {
        io.emit('nowPlayingData', data)
        previousData = data
      } else {
        io.emit('nowPlayingData', previousData)
      }
    } else if (!song || !song.is_playing) {
      if (!isEqual(previousData, '{"is_playing":false}')) {
        io.emit('nowPlayingData', '{"is_playing":false}')
        previousData = '{"is_playing":false}'
      }
    }
  } catch (error) {
    console.error('Error fetching song data:', error)
  }
}

function isEqual(obj1: string, obj2: string): boolean {
  return JSON.stringify(obj1) === JSON.stringify(obj2)
}

server.on('request', (req: http.IncomingMessage, res: http.ServerResponse) => {
  if (req.url === '/') {
    res.writeHead(426, { 'Content-Type': 'text/plain' })
    res.end('Upgrade Required')
  } else {
    return
  }
})

io.on('connection', (socket: Socket) => {
  if (previousData) {
    socket.emit('nowPlayingData', previousData)
  }

  const intervalId = setInterval(() => {
    sendNowPlayingData().catch((err) => {
      console.error('Error sending NowPlayingData:', err)
    })
  }, 3000)

  socket.on('disconnect', () => {
    clearInterval(intervalId)
  })

  socket.on('error', (err) => {
    console.error('WebSocket error:', err)
  })
})

server.listen(3000, () => {
  console.log(`\u001b[1;32m[SPOTIFY-WS] \x1b[34mSTARTED ON PORT 3000`)
})
