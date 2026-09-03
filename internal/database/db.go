package database

import (
	"context"
	"time"

	"github.com/enterprise/distributed-search-service/internal/models"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	_ "github.com/lib/pq"
)

type DB struct {
	conn *sqlx.DB
}

func NewDB(dsn string) (*DB, error) {
	conn, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		return nil, err
	}
	conn.SetMaxOpenConns(50)
	conn.SetMaxIdleConns(10)
	conn.SetConnMaxLifetime(5 * time.Minute)
	return &DB{conn: conn}, nil
}

func (d *DB) Ping(ctx context.Context) error {
	return d.conn.PingContext(ctx)
}

func (d *DB) InsertDocument(ctx context.Context, doc *models.Document) error {
	query := `
		INSERT INTO documents (tenant_id, document_id, title, body, status, tags, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
		ON CONFLICT (tenant_id, document_id) DO UPDATE 
		SET title = EXCLUDED.title, body = EXCLUDED.body, status = EXCLUDED.status, 
		    tags = EXCLUDED.tags, updated_at = NOW()`
	_, err := d.conn.ExecContext(ctx, query, doc.TenantID, doc.ID, doc.Title, doc.Body, doc.Status, pq.Array(doc.Tags))
	return err
}

func (d *DB) UpdateDocumentStatus(ctx context.Context, tenantID, docID, status string) error {
	query := `UPDATE documents SET status = $1, updated_at = NOW() WHERE tenant_id = $2 AND document_id = $3`
	_, err := d.conn.ExecContext(ctx, query, status, tenantID, docID)
	return err
}

func (d *DB) GetDocument(ctx context.Context, tenantID, docID string) (*models.Document, error) {
	var doc models.Document
	query := `SELECT tenant_id, document_id, title, body, status, tags, created_at, updated_at 
	          FROM documents WHERE tenant_id = $1 AND document_id = $2`
	err := d.conn.QueryRowxContext(ctx, query, tenantID, docID).Scan(
		&doc.TenantID, &doc.ID, &doc.Title, &doc.Body, &doc.Status, pq.Array(&doc.Tags), &doc.CreatedAt, &doc.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

func (d *DB) DeleteDocument(ctx context.Context, tenantID, docID string) error {
	query := `DELETE FROM documents WHERE tenant_id = $1 AND document_id = $2`
	_, err := d.conn.ExecContext(ctx, query, tenantID, docID)
	return err
}
