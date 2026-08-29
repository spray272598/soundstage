package vectorstore

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	aidomain "github.com/spray272598/soundstage/internal/ai/domain"
)

// PGVectorStore implements VectorStore using PostgreSQL with pgvector extension.
type PGVectorStore struct {
	pool       *pgxpool.Pool
	tableName  string
	vectorDims int
}

// NewPGVectorStore creates a new pgvector-backed vector store.
// It expects the pgvector extension to be installed and the table to exist.
func NewPGVectorStore(ctx context.Context, cfg aidomain.PGVectorConfig) (*PGVectorStore, error) {
	if cfg.DSN == "" {
		return nil, fmt.Errorf("pgvector: dsn is required")
	}
	if cfg.TableName == "" {
		cfg.TableName = "ai_knowledge_base"
	}
	if cfg.VectorDims == 0 {
		cfg.VectorDims = 1536
	}
	if cfg.PoolSize == 0 {
		cfg.PoolSize = 10
	}
	if cfg.HNSWEfSearch == 0 {
		cfg.HNSWEfSearch = 64
	}

	poolConfig, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("pgvector: parse dsn: %w", err)
	}
	poolConfig.MaxConns = int32(cfg.PoolSize)

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("pgvector: connect: %w", err)
	}

	// Verify pgvector extension and table
	if err := ensureSchema(ctx, pool, cfg.TableName, cfg.VectorDims); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pgvector: ensure schema: %w", err)
	}

	// Set HNSW ef_search for this session (can also be set per-query)
	if _, err := pool.Exec(ctx, fmt.Sprintf("SET hnsw.ef_search = %d", cfg.HNSWEfSearch)); err != nil {
		// Non-fatal, just log
		// logger.L().Warn("pgvector: set hnsw.ef_search failed", zap.Error(err))
	}

	return &PGVectorStore{
		pool:       pool,
		tableName:  cfg.TableName,
		vectorDims: cfg.VectorDims,
	}, nil
}

func ensureSchema(ctx context.Context, pool *pgxpool.Pool, tableName string, dims int) error {
	// Check pgvector extension
	var extExists bool
	err := pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = 'vector')").Scan(&extExists)
	if err != nil {
		return fmt.Errorf("check vector extension: %w", err)
	}
	if !extExists {
		// Try to create (requires superuser)
		if _, err := pool.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS vector"); err != nil {
			return fmt.Errorf("create vector extension: %w (run as superuser: CREATE EXTENSION vector)", err)
		}
	}

	// Create table if not exists
	createTableSQL := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id TEXT PRIMARY KEY,
			vector vector(%d) NOT NULL,
			payload JSONB NOT NULL DEFAULT '{}',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS %s_vector_idx ON %s USING hnsw (vector vector_cosine_ops);
		CREATE INDEX IF NOT EXISTS %s_payload_idx ON %s USING gin (payload);
	`, tableName, dims, tableName, tableName, tableName, tableName)

	_, err = pool.Exec(ctx, createTableSQL)
	return err
}

// Upsert inserts or updates vectors.
func (s *PGVectorStore) Upsert(ctx context.Context, points []aidomain.VectorPoint) error {
	if len(points) == 0 {
		return nil
	}

	// Build batched upsert using ON CONFLICT
	valueArgs := make([]string, 0, len(points))
	args := make([]any, 0, len(points)*3)
	argIdx := 1

	for _, p := range points {
		if len(p.Vector) != s.vectorDims {
			return fmt.Errorf("vector dims mismatch: got %d, expected %d", len(p.Vector), s.vectorDims)
		}
		payloadJSON, err := json.Marshal(p.Payload)
		if err != nil {
			return fmt.Errorf("marshal payload: %w", err)
		}

		valueArgs = append(valueArgs, fmt.Sprintf("($%d, $%d, $%d)", argIdx, argIdx+1, argIdx+2))
		args = append(args, p.ID, fmt.Sprintf("[%s]", vectorToString(p.Vector)), string(payloadJSON))
		argIdx += 3
	}

	sql := fmt.Sprintf(`
		INSERT INTO %s (id, vector, payload)
		VALUES %s
		ON CONFLICT (id) DO UPDATE SET
			vector = EXCLUDED.vector,
			payload = EXCLUDED.payload,
			updated_at = NOW()
	`, s.tableName, strings.Join(valueArgs, ", "))

	_, err := s.pool.Exec(ctx, sql, args...)
	return err
}

func vectorToString(v []float32) string {
	strs := make([]string, len(v))
	for i, val := range v {
		strs[i] = fmt.Sprintf("%.6f", val)
	}
	return strings.Join(strs, ",")
}

// Search returns topK most similar vectors using cosine similarity.
func (s *PGVectorStore) Search(ctx context.Context, query []float32, topK int) ([]aidomain.VectorHit, error) {
	if len(query) != s.vectorDims {
		return nil, fmt.Errorf("query vector dims mismatch: got %d, expected %d", len(query), s.vectorDims)
	}
	if topK <= 0 {
		topK = 10
	}

	queryStr := fmt.Sprintf("[%s]", vectorToString(query))

	sql := fmt.Sprintf(`
		SELECT id, vector <=> $1 AS distance, payload
		FROM %s
		ORDER BY distance ASC
		LIMIT $2
	`, s.tableName)

	rows, err := s.pool.Query(ctx, sql, queryStr, topK)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	defer rows.Close()

	hits := make([]aidomain.VectorHit, 0, topK)
	for rows.Next() {
		var id string
		var distance float32
		var payloadJSON []byte
		if err := rows.Scan(&id, &distance, &payloadJSON); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}

		var payload map[string]any
		if err := json.Unmarshal(payloadJSON, &payload); err != nil {
			payload = map[string]any{}
		}

		// Convert distance to similarity score (cosine distance -> cosine similarity)
		score := 1.0 - distance
		hits = append(hits, aidomain.VectorHit{
			ID:      id,
			Score:   score,
			Payload: payload,
		})
	}
	return hits, rows.Err()
}

// Delete removes vectors by ID.
func (s *PGVectorStore) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	sql := fmt.Sprintf(`DELETE FROM %s WHERE id IN (%s)`, s.tableName, strings.Join(placeholders, ", "))
	_, err := s.pool.Exec(ctx, sql, args...)
	return err
}

// Count returns the number of stored vectors.
func (s *PGVectorStore) Count(ctx context.Context) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", s.tableName)).Scan(&count)
	return count, err
}

// Close releases the connection pool.
func (s *PGVectorStore) Close() error {
	s.pool.Close()
	return nil
}

// Compile-time check.
var _ aidomain.VectorStore = (*PGVectorStore)(nil)

// Helper for tests: create a test database with pgvector.
// Requires: docker run -d -p 5432:5432 -e POSTGRES_PASSWORD=postgres pgvector/pgvector:pg16
func NewTestPGVectorStore(ctx context.Context, dsn string) (*PGVectorStore, func(), error) {
	cfg := aidomain.PGVectorConfig{
		DSN:         dsn,
		TableName:   "ai_knowledge_base_test",
		VectorDims:  32, // Use 32 dims for mock embedder compatibility
		PoolSize:    5,
	}
	store, err := NewPGVectorStore(ctx, cfg)
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() {
		// Drop test table
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		store.pool.Exec(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", cfg.TableName))
		store.Close()
	}
	return store, cleanup, nil
}