package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	apphttp "m-v-k/backend/internal/http"
	"m-v-k/backend/internal/http/handlers"
	"m-v-k/backend/internal/word"
	"m-v-k/backend/internal/word/store"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "3003"
	}

	workingDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("read working directory failed: %v", err)
	}

	wordStore, err := store.NewJSONStore(
		filepath.Join(workingDir, "full.json"),
		filepath.Join(workingDir, "next.json"),
	)
	if err != nil {
		log.Fatalf("initialize json store failed: %v", err)
	}

	wordService := word.NewService(wordStore, 2048)
	wordHandler := handlers.NewWordHandler(wordService)
	router := apphttp.NewRouter(wordHandler)

	addr := ":" + port
	log.Printf("backend server listening on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}
