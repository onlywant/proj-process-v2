package main

import (
	"log"
	"os"

	"proj-process-v2/backend/internal/app"
)

func main() {
	addr := env("ADDR", ":8080")
	dbPath := env("DB_PATH", "data/app.db")
	frontendDir := env("FRONTEND_DIR", "../frontend/dist")
	srv, err := app.New(dbPath, frontendDir)
	if err != nil { log.Fatal(err) }
	log.Printf("listening on %s", addr)
	log.Fatal(srv.ListenAndServe(addr))
}

func env(key, fallback string) string { if value := os.Getenv(key); value != "" { return value }; return fallback }
