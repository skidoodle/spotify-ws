# Prepare
FROM node:alpine AS builder
WORKDIR /app

ENV PNPM_HOME="/pnpm"
ENV PATH="$PNPM_HOME:$PATH"

RUN npm install -g pnpm \
    && SHELL=bash pnpm setup \
    && pnpm install -g typescript pm2

COPY package.json pnpm-lock.yaml ./
RUN pnpm install
COPY . .
RUN tsc src/server.ts --outDir dist

# Final
FROM builder AS final 
WORKDIR /app
COPY --from=builder /app/dist ./dist
COPY --from=builder /app/node_modules ./node_modules

# Run
CMD pm2 start process.yml && tail -f /dev/null
EXPOSE 3000