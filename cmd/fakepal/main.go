package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/elug3/fakepal/internal/api"
	"github.com/elug3/fakepal/internal/auth"
	"github.com/elug3/fakepal/internal/config"
	"github.com/elug3/fakepal/internal/store"
)

func main() {
	cfg := config.Load()
	addr := cfg.Port
	if !strings.HasPrefix(addr, ":") {
		addr = ":" + addr
	}

	mem := store.NewMemoryStore()
	baseURL := fmt.Sprintf("http://localhost%s", addr)
	srv := api.NewServer(mem, baseURL)
	handler := auth.Middleware(cfg.APIKey)(srv.Handler())

	log.Printf("fakepal listening on %s (api key configured)", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatal(err)
	}
}
