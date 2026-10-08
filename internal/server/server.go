package server

import (
	"net/http"
	"time"

	"github.com/PathumSandeepa/iconfigly-api/internal/health"
)

func New(port string) *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", health.Handler)

	return &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
}