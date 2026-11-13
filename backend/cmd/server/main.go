package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	apphttp "mgl-ime-core/backend/internal/http"
	"mgl-ime-core/backend/internal/http/handlers"
	"mgl-ime-core/backend/internal/word"
	"mgl-ime-core/backend/internal/word/store"
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

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server stopped: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Printf("shutdown signal received, draining requests...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("graceful shutdown failed: %v", err)
	}
	log.Printf("server shut down")
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
