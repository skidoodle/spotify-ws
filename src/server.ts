import * as http from 'http'
import { Server as SocketIOServer, type Socket } from 'socket.io'
import 'dotenv/config'

const HOST = process.env.HOST ?? 'http://localhost'
const PORT = process.env.PORT ?? 3000
const ENDPOINT = process.env.ENDPOINT

const server = http.createServer()
const io = new SocketIOServer(server, {
  cors: {
    origin: '*',
  },
})

let previousData: string

const sendNowPlayingData = async () => {
  try {
    const response = await fetch(`${ENDPOINT}`)
    const data = (await response.json()) as string

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

function isEqual(obj1: string, obj2: string) {
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

server.listen(PORT, () => {
  console.log(`[SPOTIFY-WS] STARTED - ${HOST}:${PORT}`)
})
