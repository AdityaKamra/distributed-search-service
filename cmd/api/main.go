package main

import (
	"log"
	"net/http"

	"github.com/enterprise/distributed-search-service/internal/api"
	"github.com/enterprise/distributed-search-service/internal/cache"
	"github.com/enterprise/distributed-search-service/internal/config"
	"github.com/enterprise/distributed-search-service/internal/database"
	"github.com/enterprise/distributed-search-service/internal/queue"
	"github.com/enterprise/distributed-search-service/internal/search"
)

func main() {
	cfg := config.LoadConfig()
	log.Println("Starting Distributed Document Search API Gateway...")

	db, err := database.NewDB(cfg.PostgresDSN)
	if err != nil {
		log.Fatalf("Fatal: Database initialization failed: %v", err)
	}

	redisCache, err := cache.NewCache(cfg.RedisAddr)
	if err != nil {
		log.Fatalf("Fatal: Redis initialization failed: %v", err)
	}

	searchEngine, err := search.NewSearchEngine(cfg.ESURL)
	if err != nil {
		log.Fatalf("Fatal: Elasticsearch initialization failed: %v", err)
	}

	kafkaProducer := queue.NewProducer(cfg.KafkaBrokers, "doc.index.v1")
	defer kafkaProducer.Close()

	handler := api.NewHandler(db, redisCache, searchEngine, kafkaProducer)
	router := api.SetupRouter(handler)

	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	log.Printf("Search API running on port %s", cfg.Port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server shutdown error: %v", err)
	}
}
