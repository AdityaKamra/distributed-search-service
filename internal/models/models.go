package models

import "time"

type Document struct {
	TenantID  string    `db:"tenant_id" json:"tenant_id"`
	ID        string    `db:"document_id" json:"id"`
	Title     string    `db:"title" json:"title"`
	Body      string    `db:"body" json:"body"`
	Status    string    `db:"status" json:"status"`
	Tags      []string  `db:"tags" json:"tags"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

type CreateDocumentRequest struct {
	ID    string   `json:"id"`
	Title string   `json:"title" binding:"required"`
	Body  string   `json:"body" binding:"required"`
	Tags  []string `json:"tags"`
}

type SearchResultHit struct {
	ID        string   `json:"id"`
	Score     float64  `json:"score"`
	Title     string   `json:"title"`
	Body      string   `json:"body"`
	Tags      []string `json:"tags"`
	Highlight []string `json:"highlight,omitempty"`
}

type SearchResponse struct {
	TookMs    int64             `json:"took_ms"`
	Cached    bool              `json:"cached"`
	TotalHits int64             `json:"total_hits"`
	Hits      []SearchResultHit `json:"hits"`
}

type KafkaIndexMessage struct {
	TenantID   string    `json:"tenant_id"`
	DocumentID string    `json:"document_id"`
	Title      string    `json:"title"`
	Body       string    `json:"body"`
	Tags       []string  `json:"tags"`
	Timestamp  time.Time `json:"timestamp"`
}
