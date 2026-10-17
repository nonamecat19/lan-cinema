# Local TV Series Streaming App

A self-hosted TV app for browsing and streaming a local series library. The Go server prepares compatible MP4 files, serves the catalog and byte-range streams, and hosts the React app on the same origin. Docker Compose can also run qBittorrent as a private download engine controlled from the app.

## Requirements

- Linux host on the same trusted local network as the TV
- Go toolchain
- Node.js 20.19+ or 22.12+ and npm
- FFmpeg and FFprobe available on `PATH`
- Read access to the media library and write access to a separate cache directory

## Build

Build the frontend, then build the server from the repository root:

```sh
cd web
npm ci
npm run build
cd ..
go build -buildvcs=false -o tv-streamer .
```

The frontend build is written to `web/dist`, which the server serves for browser routes.

## Run

Set absolute paths for `MEDIA_ROOT` and `CACHE_DIR`. The cache must not be inside the media root. `PORT` defaults to `8080`.

```sh
MEDIA_ROOT=/srv/media/series CACHE_DIR=/var/cache/tv-streamer PORT=8080 ./tv-streamer
```

Open the LAN URL printed by the server in the TV browser. The listening port must be reachable over the trusted LAN. Do not forward it to the internet; the app has no authentication.

The startup scan recognizes series directories containing `Season NN` directories and files named `SNNENN - Title` with MP4, MKV, AVI, MOV, M4V, or WebM extensions. Manually added files are picked up after a server restart. Completed torrent downloads are imported and added to the catalog automatically. Unsupported codecs are prepared as H.264/AAC MP4 in the cache. The library opens after scanning while media preparation continues in the background. Episode rows show probing, remuxing, conversion, validation, and failure status.

Progress is stored in each browser's local storage and is not synchronized between devices. Unfinished episodes appear in the Continue watching shelf, ordered by the most recent playback. Media preparation writes only to the separate cache. Torrent imports create normalized copies or hard links in the library while preserving the originals for seeding.

See [spec](spec/00-overview.md) for the complete behavior and acceptance criteria.

## Docker Compose

Docker Compose builds the frontend and Go server into one image. FFmpeg and FFprobe are included in the runtime image. By default, Compose mounts `./videos` for the media library, uses `./cache` for prepared media, and reads `.torrent` files from the project root. qBittorrent stores active downloads under `./downloads`; the app keeps the WebUI private to the Compose network.

```sh
docker compose up --build -d
docker compose logs -f tv
```

Open **Downloads** in the app to upload a `.torrent` file from your device or select one from the project root or `TORRENT_EXTRA_HOST_PATH`, inspect its contents, choose seasons, and start, pause, or remove transfers. Removing a transfer deletes its qBittorrent staging data while imported episodes stay in the media library. Uploaded torrent files persist in `./cache/torrent-uploads` and are limited to 8 MB. The app imports each completed video into the matching series and season folder in `MEDIA_HOST_PATH`, preserving qBittorrent's original files for seeding. It prepares imported MKV and other supported video files for browser playback and refreshes the catalog automatically.

The Downloads page shows transfer state and per-episode download, waiting import, active copy progress, import errors, and media preparation status. Import errors remain visible after a restart and clear when a later attempt succeeds. A series imported from a selected torrent shows that torrent on its series page. If the torrent contains seasons that have not been selected, use **More seasons** there to add them to the existing transfer.

Put additional `.torrent` files beside `compose.yaml`, then reload the Downloads screen. Torrent data and client settings persist in `./downloads` and `./qbt-config`. The qBittorrent WebUI is not published on the host. TCP and UDP port `6881` are published for peer connections; allow them through the host firewall if downloads need incoming peers.

Wait for the `startup ready` log, then open `http://<server-lan-address>:8080` on the TV. Find the address of the host's network interface with `ip -4 -o addr show scope global`; choose the address on the same network as the TV, not a Docker or VPN address. Set `LAN_URL` in `.env` to that full URL so the startup log prints the TV address rather than the container address. Allow TCP port `8080` from the trusted LAN in the host firewall. Do not forward the port to the internet.

Set these variables in a `.env` file beside `compose.yaml` to use other host paths or a different host port:

```env
MEDIA_HOST_PATH=/srv/media/series
CACHE_HOST_PATH=/var/cache/tv-streamer
HOST_PORT=8080
LAN_URL=http://192.168.1.10:8080
TORRENT_HOST_PATH=.
DOWNLOAD_HOST_PATH=./downloads
QBITTORRENT_CONFIG_PATH=./qbt-config
```

Stop the app with `docker compose down`.

## Code layout

- `server/catalog.go` scans the media library and builds the public catalog.
- `server/media_probe.go`, `server/media_preview.go`, and `server/prepare.go` probe, preview, and prepare playable media.
- `server/http.go` serves the catalog, media, and frontend. `server/http_downloads.go` exposes torrent and transfer actions.
- `server/torrent.go`, `server/torrent_client.go`, `server/torrent_metadata.go`, and `server/torrent_state.go` own the download workflow, qBittorrent requests, metadata parsing, and persisted selections.
- `web/src/App.tsx` chooses the current screen. `web/src/screens` holds screen views, `web/src/features` holds feature state and components, `web/src/domain` defines API models and validates responses, `web/src/remote` owns TV key and focus behavior, and `web/src/hooks` owns routing and catalog polling.

Run `go test -race ./...` for backend checks. In `web`, run `npm test` for remote navigation tests and `npm run build` for strict TypeScript checking and the production bundle.

The remote uses arrows or digits 2/4/6/8 to move, OK or 5 to select, and Back or 0 to return. The selected season and focused episode are restored when returning from playback.

The player requests browser fullscreen after an episode is selected. If the TV browser rejects fullscreen, it remains usable in the viewport. Follow the [TV acceptance procedure](spec/04-verification.md#tv-acceptance) on the intended TV and record the results; automated tests do not verify physical remote behavior or the three-second first-frame target.
