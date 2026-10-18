FROM node:26-alpine AS frontend
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.23-alpine AS backend
WORKDIR /src
COPY go.mod ./go.mod
COPY main.go ./main.go
COPY server ./server
RUN CGO_ENABLED=0 go build -buildvcs=false -o /out/tv-streamer .

FROM alpine:3.22
RUN apk add --no-cache ca-certificates ffmpeg mesa-va-gallium
WORKDIR /app
COPY --from=backend /out/tv-streamer /app/tv-streamer
COPY --from=frontend /src/web/dist /app/web/dist
ENV PORT=8080
EXPOSE 8080
ENTRYPOINT ["/app/tv-streamer"]
