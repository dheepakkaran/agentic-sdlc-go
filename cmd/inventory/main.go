// Command inventory serves the network device inventory API.
package main

import (
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/dheepakkaran/go-ai-sdlc-pipeline/internal/inventory"
)

func main() {
	addr := ":8080"
	if v := os.Getenv("ADDR"); v != "" {
		if _, _, err := net.SplitHostPort(v); err != nil {
			log.Fatal("ADDR must be host:port")
		}
		addr = v
	}
	srv := &http.Server{
		Addr:              addr,
		Handler:           inventory.Handler(inventory.NewStore()),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Print("inventory listening on " + strconv.Quote(addr))
	log.Fatal(srv.ListenAndServe())
}
