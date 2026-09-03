#!/bin/sh
set -e

BASE_URL="http://localhost:8080"
TENANT_A="tnt_apple"
TENANT_B="tnt_google"

echo "=================================================="
echo "1. Checking Health Status..."
echo "=================================================="
curl -s "${BASE_URL}/health"
echo ""

echo "=================================================="
echo "2. Ingesting Document for Tenant A..."
echo "=================================================="
curl -s -X POST "${BASE_URL}/documents" \
  -H "X-Tenant-ID: ${TENANT_A}" \
  -H "Content-Type: application/json" \
  -d '{"id": "doc_a_101", "title": "Distributed Systems Consensus", "body": "Raft consensus, Paxos protocol, and sharding.", "tags": ["distributed-systems", "raft"]}'
echo ""

echo "=================================================="
echo "3. Ingesting Document for Tenant B (Secret Check)..."
echo "=================================================="
curl -s -X POST "${BASE_URL}/documents" \
  -H "X-Tenant-ID: ${TENANT_B}" \
  -H "Content-Type: application/json" \
  -d '{"id": "doc_b_202", "title": "Secret Roadmap", "body": "Autonomous vehicle model weights.", "tags": ["secret"]}'
echo ""

echo "Waiting 4s for Kafka worker indexing..."
sleep 4

echo "=================================================="
echo "4. Point Lookup via API (Tenant A)..."
echo "=================================================="
curl -s -X GET "${BASE_URL}/documents/doc_a_101" -H "X-Tenant-ID: ${TENANT_A}"
echo ""

echo "=================================================="
echo "5. Searching Documents (Tenant A)..."
echo "=================================================="
curl -s -X GET "${BASE_URL}/search?q=consensus" -H "X-Tenant-ID: ${TENANT_A}"
echo ""

echo "=================================================="
echo "6. Multi-Tenant Leakage Verification..."
echo "=================================================="
LEAK=$(curl -s -X GET "${BASE_URL}/search?q=Secret" -H "X-Tenant-ID: ${TENANT_A}")
echo "$LEAK"
echo "$LEAK" | grep -q '"total_hits":0' && echo "-> ISOLATION VERIFIED: 0 hits leaked!" || (echo "FAILED ISOLATION" && exit 1)

echo "=================================================="
echo "7. Deleting Document..."
echo "=================================================="
curl -s -X DELETE "${BASE_URL}/documents/doc_a_101" -H "X-Tenant-ID: ${TENANT_A}"
echo ""

echo "=================================================="
echo "ALL VERIFICATION TESTS COMPLETED SUCCESSFULLY!"
echo "=================================================="
