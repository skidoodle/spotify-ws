FROM node:alpine
WORKDIR /app
COPY package.json .
RUN npm install && npm install typescript pm2 -g
COPY . .
RUN tsc src/server.ts --outDir dist
CMD pm2 start process.yml && tail -f /dev/null
EXPOSE 3000