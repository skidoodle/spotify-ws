FROM node:alpine
WORKDIR /app
ENV PNPM_HOME="/pnpm"
ENV PATH="$PNPM_HOME:$PATH"
RUN npm install -g pnpm \
  && SHELL=bash pnpm setup \
  && pnpm install typescript
COPY package.json pnpm-lock.yaml ./
RUN pnpm install
COPY . .
EXPOSE 3000
USER node
CMD ["pnpm", "serve"]