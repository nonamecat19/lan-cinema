package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"path/filepath"

	"hisense/server"
)

func main() {
	config, err := server.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}
	app := server.NewApp(filepath.Join("web", "dist"))
	torrents := server.NewTorrentManager(config.TorrentRoot, config.TorrentExtraRoot, config.MediaRoot, config.TorrentDownloadRoot, config.TorrentURL, filepath.Join(config.CacheDir, "torrent-state.json"))
	app.ConfigureLibrary(config.MediaRoot, config.CacheDir, torrents)
	if err := app.RefreshCatalog(); err != nil {
		log.Fatalf("unable to scan MEDIA_ROOT: %v", err)
	}
	address := ":" + config.Port
	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("unable to listen on configured port: %v", err)
	}
	torrents.StartWatcher(app.RefreshCatalog)
	if config.LANURL != "" {
		log.Printf("startup ready at %s", config.LANURL)
	} else {
		log.Printf("startup ready at %s", lanURL(config.Port))
	}
	if err := http.Serve(listener, app.Handler()); err != nil {
		log.Fatal(err)
	}
}

func lanURL(port string) string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return fmt.Sprintf("http://localhost:%s", port)
	}
	for _, networkInterface := range interfaces {
		if networkInterface.Flags&net.FlagUp == 0 || networkInterface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, err := networkInterface.Addrs()
		if err != nil {
			continue
		}
		for _, address := range addresses {
			ip, _, err := net.ParseCIDR(address.String())
			if err == nil && ip.To4() != nil {
				return fmt.Sprintf("http://%s:%s", ip.String(), port)
			}
		}
	}
	return fmt.Sprintf("http://localhost:%s", port)
}
