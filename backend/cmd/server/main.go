package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

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

	dataDir, err := resolveDataDir()
	if err != nil {
		log.Fatalf("resolve data directory failed: %v", err)
	}

	fullPath, err := resolveDataFile(dataDir, "FULL_JSON", "full.json")
	if err != nil {
		log.Fatalf("resolve full json failed: %v", err)
	}
	nextPath, err := resolveDataFile(dataDir, "NEXT_JSON", "next.json")
	if err != nil {
		log.Fatalf("resolve next json failed: %v", err)
	}

	userFullPath := filepath.Join(dataDir, "user_full.json")
	userNextPath := filepath.Join(dataDir, "user_next.json")
	if path := os.Getenv("USER_FULL_JSON"); path != "" {
		userFullPath = path
	}
	if path := os.Getenv("USER_NEXT_JSON"); path != "" {
		userNextPath = path
	}

	wordStore, err := store.NewJSONStoreWithUser(fullPath, nextPath, userFullPath, userNextPath)
	if err != nil {
		log.Fatalf("initialize json store failed: %v", err)
	}

	wordService := word.NewService(wordStore, 2048)
	wordHandler := handlers.NewWordHandler(wordService)
	router := apphttp.NewRouter(wordHandler)

	addr := ":" + port
	server := &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}

	log.Printf("loaded word data from %s and %s (user overlay: %s, %s)", fullPath, nextPath, userFullPath, userNextPath)
	log.Printf("backend server listening on %s", addr)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}

func resolveDataDir() (string, error) {
	if dir := os.Getenv("DATA_DIR"); dir != "" {
		return dir, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	if _, err := store.ResolveDataPath(cwd, "full.json"); err == nil {
		return cwd, nil
	}

	nested := filepath.Join(cwd, "backend")
	if _, err := store.ResolveDataPath(nested, "full.json"); err == nil {
		return nested, nil
	}

	return cwd, nil
}

func resolveDataFile(dir, envName, defaultName string) (string, error) {
	if path := os.Getenv(envName); path != "" {
		return path, nil
	}
	return store.ResolveDataPath(dir, defaultName)
}
