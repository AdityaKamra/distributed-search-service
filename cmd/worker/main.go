package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/enterprise/distributed-search-service/internal/cache"
	"github.com/enterprise/distributed-search-service/internal/config"
	"github.com/enterprise/distributed-search-service/internal/database"
	"github.com/enterprise/distributed-search-service/internal/models"
	"github.com/enterprise/distributed-search-service/internal/search"
	"github.com/segmentio/kafka-go"
)

func main() {
	cfg := config.LoadConfig()
	log.Println("Initializing Indexer Worker Daemon...")

	db, err := database.NewDB(cfg.PostgresDSN)
	if err != nil {
		log.Fatalf("Worker DB Init Failed: %v", err)
	}

	searchEngine, err := search.NewSearchEngine(cfg.ESURL)
	if err != nil {
		log.Fatalf("Worker SearchEngine Init Failed: %v", err)
	}

	redisCache, err := cache.NewCache(cfg.RedisAddr)
	if err != nil {
		log.Fatalf("Worker Cache Init Failed: %v", err)
	}

	// Connect directly to partition 0 to bypass consumer group rebalance locks
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:   []string{cfg.KafkaBrokers},
		Topic:     "doc.index.v1",
		Partition: 0,
		MinBytes:  1,
		MaxBytes:  10e6,
	})
	defer reader.Close()

	// Start reading from the very beginning of the topic
	if err := reader.SetOffset(kafka.FirstOffset); err != nil {
		log.Printf("Notice: SetOffset error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Stopping worker...")
		cancel()
	}()

	log.Println("Kafka Indexer Worker ACTIVE and polling partition 0 from offset 0...")

	for {
		msg, err := reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			log.Printf("Worker read error: %v", err)
			time.Sleep(500 * time.Millisecond)
			continue
		}

		log.Printf("==> Consumed message at offset %d: key=%s", msg.Offset, string(msg.Key))

		var payload models.KafkaIndexMessage
		if err := json.Unmarshal(msg.Value, &payload); err != nil {
			log.Printf("JSON unmarshal error: %v", err)
			continue
		}

		doc := &models.Document{
			TenantID:  payload.TenantID,
			ID:        payload.DocumentID,
			Title:     payload.Title,
			Body:      payload.Body,
			Tags:      payload.Tags,
			Status:    "ACTIVE",
			UpdatedAt: time.Now(),
		}

		if err := searchEngine.IndexDocument(ctx, doc); err != nil {
			log.Printf("Elasticsearch index error for doc %s: %v", doc.ID, err)
			continue
		}

		if err := db.UpdateDocumentStatus(ctx, payload.TenantID, payload.DocumentID, "ACTIVE"); err != nil {
			log.Printf("PostgreSQL update error for doc %s: %v", doc.ID, err)
			continue
		}

		redisCache.InvalidateTenantCache(ctx, payload.TenantID)
		log.Printf("SUCCESS: Document %s transitioned to ACTIVE for tenant %s", payload.DocumentID, payload.TenantID)
	}
}
