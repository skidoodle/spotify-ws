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
  path: '/',
})

const spotify = new SpotifyService({
  clientId: CLIENT_ID,
  clientSecret: CLIENT_SECRET,
  refreshToken: REFRESH_TOKEN,
})

const sendNowPlayingData = async () => {
  try {
    const song = await spotify.getCurrentSong()

    if (song && song.is_playing) {
      const data = JSON.stringify(song)
      io.emit('nowPlayingData', data)
      sendNowPlayingData.playing = true
    } else if (sendNowPlayingData.playing) {
      io.emit('nowPlayingData', null)
      sendNowPlayingData.playing = false
    }
  } catch (error) {
    console.error('Error fetching song data:', error)
  }
}

sendNowPlayingData.playing = false

io.on('connection', async (socket: Socket) => {
  await sendNowPlayingData()

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
  console.log(`\u001b[1;32m[SPOTIFY-WS] \x1b[34mSTARTED ON PORT 3000\x1b[0m\n`)
})
