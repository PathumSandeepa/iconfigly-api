package main

import (
	"log"
	"net/http"

	"github.com/PathumSandeepa/iconfigly-api/internal/config"
	"github.com/PathumSandeepa/iconfigly-api/internal/database"
	"github.com/PathumSandeepa/iconfigly-api/internal/server"
)

func main() {
	cfg, err := config.Load()

	if err != nil {
		log.Fatal(err)
	}

	db, err := database.NewPostgres(cfg.DatabaseURL)

	if err != nil {
		log.Fatal("failed to connect to PostgreSQL: ", err)
	}

	defer db.Close()

	appServer := server.New(cfg.Port)

	log.Println("iconfigly-api running on :" + cfg.Port)

	if err := appServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}