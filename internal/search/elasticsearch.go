package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/enterprise/distributed-search-service/internal/models"
)

type SearchEngine struct {
	client *elasticsearch.Client
	index  string
}

func NewSearchEngine(url string) (*SearchEngine, error) {
	cfg := elasticsearch.Config{Addresses: []string{url}}
	client, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	se := &SearchEngine{client: client, index: "documents"}
	if err := se.ensureIndexExists(); err != nil {
		return nil, err
	}
	return se, nil
}

func (s *SearchEngine) Ping(ctx context.Context) error {
	res, err := s.client.Info(s.client.Info.WithContext(ctx))
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.IsError() {
		return fmt.Errorf("elasticsearch info status: %s", res.Status())
	}
	return nil
}

func (s *SearchEngine) ensureIndexExists() error {
	res, err := s.client.Indices.Exists([]string{s.index})
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode == 200 {
		return nil
	}

	mapping := `{
		"mappings": {
			"_routing": { "required": true },
			"properties": {
				"tenant_id": { "type": "keyword" },
				"document_id": { "type": "keyword" },
				"title": { "type": "text" },
				"body": { "type": "text" },
				"tags": { "type": "keyword" },
				"created_at": { "type": "date" }
			}
		}
	}`

	createRes, err := s.client.Indices.Create(
		s.index,
		s.client.Indices.Create.WithBody(strings.NewReader(mapping)),
	)
	if err != nil {
		return err
	}
	defer createRes.Body.Close()
	return nil
}

func (s *SearchEngine) IndexDocument(ctx context.Context, doc *models.Document) error {
	data, err := json.Marshal(doc)
	if err != nil {
		return err
	}

	res, err := s.client.Index(
		s.index,
		bytes.NewReader(data),
		s.client.Index.WithDocumentID(doc.ID),
		s.client.Index.WithRouting(doc.TenantID),
		s.client.Index.WithContext(ctx),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.IsError() {
		b, _ := io.ReadAll(res.Body)
		return fmt.Errorf("indexing failed: %s", string(b))
	}
	return nil
}

func (s *SearchEngine) Search(ctx context.Context, tenantID, queryStr string) (*models.SearchResponse, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"filter": []map[string]interface{}{
					{"term": map[string]interface{}{"tenant_id": tenantID}},
				},
				"must": []map[string]interface{}{
					{
						"multi_match": map[string]interface{}{
							"query":  queryStr,
							"fields": []string{"title^3", "body"},
						},
					},
				},
			},
		},
		"highlight": map[string]interface{}{
			"fields": map[string]interface{}{
				"body": map[string]interface{}{},
			},
		},
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(query); err != nil {
		return nil, err
	}

	res, err := s.client.Search(
		s.client.Search.WithContext(ctx),
		s.client.Search.WithIndex(s.index),
		s.client.Search.WithRouting(tenantID),
		s.client.Search.WithBody(&buf),
		s.client.Search.WithTrackTotalHits(true),
	)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("search error: %s", res.Status())
	}

	var r map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return nil, err
	}

	tookMs := int64(r["took"].(float64))
	hitsObj := r["hits"].(map[string]interface{})
	total := int64(hitsObj["total"].(map[string]interface{})["value"].(float64))
	hitsList := hitsObj["hits"].([]interface{})

	var results []models.SearchResultHit
	for _, hitItem := range hitsList {
		h := hitItem.(map[string]interface{})
		source := h["_source"].(map[string]interface{})

		hit := models.SearchResultHit{
			ID:    h["_id"].(string),
			Score: h["_score"].(float64),
			Title: source["title"].(string),
			Body:  source["body"].(string),
		}

		if tagsRaw, exists := source["tags"]; exists && tagsRaw != nil {
			for _, tag := range tagsRaw.([]interface{}) {
				hit.Tags = append(hit.Tags, tag.(string))
			}
		}

		if hl, exists := h["highlight"]; exists {
			hlMap := hl.(map[string]interface{})
			if bodyHl, exists := hlMap["body"]; exists {
				for _, snippet := range bodyHl.([]interface{}) {
					hit.Highlight = append(hit.Highlight, snippet.(string))
				}
			}
		}

		results = append(results, hit)
	}

	return &models.SearchResponse{
		TookMs:    tookMs,
		Cached:    false,
		TotalHits: total,
		Hits:      results,
	}, nil
}

func (s *SearchEngine) Delete(ctx context.Context, tenantID, docID string) error {
	res, err := s.client.Delete(
		s.index,
		docID,
		s.client.Delete.WithRouting(tenantID),
		s.client.Delete.WithContext(ctx),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	return nil
}
