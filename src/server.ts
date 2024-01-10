import * as http from 'http'
import { Server as SocketIOServer, Socket } from 'socket.io'
import 'dotenv/config'

// Mostly used for local development
const HOST = process.env.HOST || 'http://localhost'
const PORT = process.env.PORT || 3000

// Spotify nowplaying api endpoint (e.g https://albert.lol/api/spotify)
const ENDPOINT = process.env.ENDPOINT

const server = http.createServer()
const io = new SocketIOServer(server, {
  cors: {
    origin: '*',
  },
})

let previousData: any = null

const sendNowPlayingData = async () => {
  try {
    const response = await fetch(`${ENDPOINT}`)
    const data = await response.json()

    if (data) {
      if (!isEqual(data, previousData)) {
        io.emit('nowPlayingData', data)
        previousData = data
      }
    } else {
      console.error('Invalid data')
    }
  } catch (error) {
    console.error('Error fetching data:', error)
  }
}

function isEqual(obj1: any, obj2: any) {
  return JSON.stringify(obj1) === JSON.stringify(obj2)
}

// server.on('request', (req: http.IncomingMessage, res: http.ServerResponse) => {
//   if (req.url === '/') {
//     res.writeHead(426, { 'Content-Type': 'text/plain' })
//     res.end('Upgrade Required')
//   } else {
//     return
//   }
// })

io.on('connection', (socket: Socket) => {
  if (previousData) {
    socket.emit('nowPlayingData', previousData)
  }

  const intervalId = setInterval(() => {
    sendNowPlayingData()
  }, 3000)

  socket.on('disconnect', () => {
    clearInterval(intervalId)
  })

  socket.on('error', (err) => {
    console.error('WebSocket error:', err)
  })
})

server.listen(PORT, () => {
  console.log(`[SPOTIFY-WS] STARTED - ${HOST}:${PORT}`)
})
