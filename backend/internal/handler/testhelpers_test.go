package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/KemenyStudio/task-manager/internal/db"
	"github.com/KemenyStudio/task-manager/internal/middleware"
)

const testDBURL = "postgres://postgres:assessment@localhost:5432/taskmanager?sslmode=disable"

// setupTestDB connects to the test database and sets db.Pool.
// Skips the test if the database is not available.
func setupTestDB(t *testing.T) {
	t.Helper()
	if db.Pool != nil {
		return
	}

	pool, err := pgxpool.New(context.Background(), testDBURL)
	if err != nil {
		t.Skipf("skipping integration test: database not available: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		t.Skipf("skipping integration test: cannot ping database: %v", err)
	}
	db.Pool = pool
}

// getTestPool returns a pgxpool.Pool connected to the test database.
// Unlike setupTestDB, this returns the pool directly (for dependency injection).
func getTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	pool, err := pgxpool.New(context.Background(), testDBURL)
	if err != nil {
		t.Skipf("skipping integration test: database not available: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		t.Skipf("skipping integration test: cannot ping database: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

// newAuthenticatedRequest creates an HTTP request with user_id in context and chi route params.
func newAuthenticatedRequest(t *testing.T, method, path, taskID string, body interface{}) *http.Request {
	t.Helper()

	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("failed to encode request body: %v", err)
		}
	}

	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")

	// Set authenticated user
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, "a1b2c3d4-e5f6-7890-abcd-ef1234567890")

	// Set chi route params
	if taskID != "" {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", taskID)
		ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	}

	return req.WithContext(ctx)
}

// newRequestWithRouteParam creates an HTTP request with chi route params (no auth).
func newRequestWithRouteParam(t *testing.T, method, path, taskID string) *http.Request {
	t.Helper()

	req := httptest.NewRequest(method, path, http.NoBody)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", taskID)
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)

	return req.WithContext(ctx)
}
