# Distributed Document Search Service

Distributed multi-tenant document search engine capable of searching millions of documents with sub-second response times, built with Go, PostgreSQL 16, Elasticsearch 8.13, Apache Kafka, and Redis 7.2.

## Quick Start (Docker)

```bash
docker compose up -d --build
```

### Health Check
```bash
curl -s http://localhost:8080/health
```

### Run Full Test Suite
```bash
./scripts/test_suite.sh
```
