package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/enterprise/distributed-search-service/internal/cache"
	"github.com/enterprise/distributed-search-service/internal/database"
	"github.com/enterprise/distributed-search-service/internal/models"
	"github.com/enterprise/distributed-search-service/internal/queue"
	"github.com/enterprise/distributed-search-service/internal/search"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	db       *database.DB
	cache    *cache.Cache
	search   *search.SearchEngine
	producer *queue.Producer
}

func NewHandler(db *database.DB, cache *cache.Cache, search *search.SearchEngine, producer *queue.Producer) *Handler {
	return &Handler{db: db, cache: cache, search: search, producer: producer}
}

func (h *Handler) RateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := c.GetHeader("X-Tenant-ID")
		if tenantID == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "X-Tenant-ID header is required"})
			return
		}

		allowed, err := h.cache.AllowRateLimit(c.Request.Context(), tenantID, 120)
		if err != nil || !allowed {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "Rate limit quota exceeded"})
			return
		}

		c.Set("tenant_id", tenantID)
		c.Next()
	}
}

func (h *Handler) IndexDocument(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	var req models.CreateDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	docID := req.ID
	if docID == "" {
		docID = uuid.New().String()
	}

	doc := &models.Document{
		TenantID: tenantID,
		ID:       docID,
		Title:    req.Title,
		Body:     req.Body,
		Tags:     req.Tags,
		Status:   "PENDING",
	}

	if err := h.db.InsertDocument(c.Request.Context(), doc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to persist document"})
		return
	}

	msg := models.KafkaIndexMessage{
		TenantID:   tenantID,
		DocumentID: docID,
		Title:      req.Title,
		Body:       req.Body,
		Tags:       req.Tags,
		Timestamp:  time.Now(),
	}

	if err := h.producer.Publish(c.Request.Context(), tenantID, msg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to queue indexing task"})
		return
	}

	h.cache.InvalidateTenantCache(c.Request.Context(), tenantID)

	c.JSON(http.StatusAccepted, gin.H{
		"document_id": docID,
		"status":      "PENDING",
	})
}

func (h *Handler) SearchDocuments(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	q := c.Query("q")
	if q == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Search query param q is required"})
		return
	}

	hash := sha256.Sum256([]byte(fmt.Sprintf("%s:%s", tenantID, q)))
	cacheKey := fmt.Sprintf("search:%s:%s", tenantID, hex.EncodeToString(hash[:]))

	if val, err := h.cache.Get(c.Request.Context(), cacheKey); err == nil && val != "" {
		var cachedResp models.SearchResponse
		if err := json.Unmarshal([]byte(val), &cachedResp); err == nil {
			cachedResp.Cached = true
			c.JSON(http.StatusOK, cachedResp)
			return
		}
	}

	results, err := h.search.Search(c.Request.Context(), tenantID, q)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if serialized, err := json.Marshal(results); err == nil {
		_ = h.cache.Set(c.Request.Context(), cacheKey, string(serialized), 2*time.Minute)
	}

	c.JSON(http.StatusOK, results)
}

func (h *Handler) GetDocument(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	docID := c.Param("id")

	doc, err := h.db.GetDocument(c.Request.Context(), tenantID, docID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Document not found"})
		return
	}

	c.JSON(http.StatusOK, doc)
}

func (h *Handler) DeleteDocument(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	docID := c.Param("id")

	if err := h.db.DeleteDocument(c.Request.Context(), tenantID, docID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database deletion failed"})
		return
	}

	_ = h.search.Delete(c.Request.Context(), tenantID, docID)
	h.cache.InvalidateTenantCache(c.Request.Context(), tenantID)

	c.JSON(http.StatusOK, gin.H{"deleted": true, "document_id": docID})
}

func (h *Handler) HealthCheck(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	deps := gin.H{
		"postgres":      "UP",
		"redis":         "UP",
		"elasticsearch": "UP",
	}

	var failed bool
	if err := h.db.Ping(ctx); err != nil {
		deps["postgres"] = "DOWN: " + err.Error()
		failed = true
	}
	if err := h.cache.Ping(ctx); err != nil {
		deps["redis"] = "DOWN: " + err.Error()
		failed = true
	}
	if err := h.search.Ping(ctx); err != nil {
		deps["elasticsearch"] = "DOWN: " + err.Error()
		failed = true
	}

	status := "UP"
	httpCode := http.StatusOK
	if failed {
		status = "DEGRADED"
		httpCode = http.StatusServiceUnavailable
	}

	c.JSON(httpCode, gin.H{
		"status":       status,
		"timestamp":    time.Now().UTC().Format(time.RFC3339),
		"dependencies": deps,
	})
}
