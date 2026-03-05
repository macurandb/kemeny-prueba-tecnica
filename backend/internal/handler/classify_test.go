package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/KemenyStudio/task-manager/internal/handler"
	"github.com/KemenyStudio/task-manager/internal/llm"
)

func TestClassifyHandler_Handle(t *testing.T) {
	pool := getTestPool(t)

	tests := []struct {
		name           string
		taskID         string
		wantStatus     int
		wantCategory   bool
		wantSummary    bool
		wantAITags     bool
		skipIfNoTasks  bool
	}{
		{
			name:       "returns 404 for non-existent task",
			taskID:     "00000000-0000-0000-0000-000000000000",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "returns error for invalid UUID",
			taskID:     "not-a-uuid",
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:          "classifies existing task successfully",
			taskID:        "", // resolved dynamically
			wantStatus:    http.StatusOK,
			wantCategory:  true,
			wantSummary:   true,
			wantAITags:    true,
			skipIfNoTasks: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskID := tt.taskID

			// Resolve dynamic task ID for happy path
			if tt.skipIfNoTasks {
				var id string
				err := pool.QueryRow(context.Background(),
					"SELECT id FROM tasks LIMIT 1",
				).Scan(&id)
				if err != nil {
					t.Skipf("no tasks in database, skipping: %v", err)
				}
				taskID = id

				// Clean up AI tags after test
				t.Cleanup(func() {
					_, _ = pool.Exec(context.Background(),
						"DELETE FROM task_tags WHERE task_id = $1 AND assigned_by = 'ai'", taskID)
				})
			}

			classifyHandler := handler.NewClassifyHandler(llm.NewMockClient(), pool)
			req := newRequestWithRouteParam(t, http.MethodPost, "/api/tasks/"+taskID+"/classify", taskID)
			w := httptest.NewRecorder()

			classifyHandler.Handle(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("Handle() status = %d, want %d. Body: %s", w.Code, tt.wantStatus, w.Body.String())
			}

			if tt.wantStatus != http.StatusOK {
				return
			}

			// Verify response body for successful classification
			var result map[string]interface{}
			if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if tt.wantCategory {
				assertFieldNotEmpty(t, result, "category")
			}
			if tt.wantSummary {
				assertFieldNotEmpty(t, result, "summary")
			}

			// Verify AI tags persisted in DB
			if tt.wantAITags {
				var aiTagCount int
				err := pool.QueryRow(context.Background(),
					"SELECT COUNT(*) FROM task_tags WHERE task_id = $1 AND assigned_by = 'ai'",
					taskID,
				).Scan(&aiTagCount)
				if err != nil {
					t.Fatalf("failed to query task_tags: %v", err)
				}
				if aiTagCount == 0 {
					t.Error("expected AI-assigned tags in task_tags, got 0")
				}
			}
		})
	}
}

// assertFieldNotEmpty checks that a JSON field exists and is non-empty.
func assertFieldNotEmpty(t *testing.T, data map[string]interface{}, field string) {
	t.Helper()
	val, ok := data[field]
	if !ok || val == nil || val == "" {
		t.Errorf("expected %q to be non-empty, got %v", field, val)
	}
}
