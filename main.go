package main

import (
	"flag"
	"gorilla-tsdb/server"
	"gorilla-tsdb/tsdb"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP server address")
	dataDir := flag.String("data", "./data", "Data directory for blocks")
	flag.Parse()

	db, err := tsdb.NewTSDB(*dataDir)
	if err != nil {
		log.Fatalf("Failed to create TSDB: %v", err)
	}
	defer db.Close()

	mdb, err := tsdb.NewMultiTSDB(*dataDir)
	if err != nil {
		log.Fatalf("Failed to create MultiTSDB: %v", err)
	}
	defer mdb.Close()

	handler := server.NewHandler(db)
	promHandler := server.NewPrometheusHandler(mdb)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	promHandler.RegisterRoutes(mux)

	srv := &http.Server{
		Addr:    *addr,
		Handler: mux,
	}

	go func() {
		log.Printf("Starting HTTP server on %s", *addr)
		log.Printf("Data directory: %s", *dataDir)
		log.Printf("Endpoints:")
		log.Printf("  POST /write             - Write a single data point")
		log.Printf("  POST /write/batch       - Write multiple data points")
		log.Printf("  GET  /query             - Query data by time range (?start=&end=&downsample=)")
		log.Printf("  GET  /stats             - Get database statistics")
		log.Printf("  POST /flush             - Flush active block to disk")
		log.Printf("  POST /api/v1/write      - Prometheus remote write")
		log.Printf("  POST /api/v1/read       - Prometheus remote read")

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("\nShutting down server...")
	if err := db.Close(); err != nil {
		log.Printf("Error closing TSDB: %v", err)
	}
	if err := mdb.Close(); err != nil {
		log.Printf("Error closing MultiTSDB: %v", err)
	}
	log.Println("Server stopped gracefully")
}
