package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/KemenyStudio/task-manager/internal/db"
	"github.com/KemenyStudio/task-manager/internal/handler"
)

// TestUpdateTask verifies update task handler behavior for multiple scenarios.
func TestUpdateTask(t *testing.T) {
	setupTestDB(t)

	tests := []struct {
		name       string
		taskID     string
		body       map[string]string
		wantStatus int
	}{
		{
			name:       "returns 404 for non-existent task",
			taskID:     "00000000-0000-0000-0000-000000000000",
			body:       map[string]string{"status": "done"},
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "returns 400 for invalid status value",
			taskID:     "22222222-2222-2222-2222-222222222222",
			body:       map[string]string{"status": "invalid_status"},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "returns 200 for valid status update",
			taskID:     "22222222-2222-2222-2222-222222222222",
			body:       map[string]string{"status": "in_progress"},
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original status for cleanup
			var originalStatus string
			if tt.wantStatus == http.StatusOK {
				err := db.Pool.QueryRow(context.Background(),
					"SELECT status FROM tasks WHERE id = $1", tt.taskID,
				).Scan(&originalStatus)
				if err != nil {
					t.Fatalf("failed to get original status: %v", err)
				}
				t.Cleanup(func() {
					restoreTaskStatus(t, tt.taskID, originalStatus)
				})
			}

			req := newAuthenticatedRequest(t, "PUT", "/api/tasks/"+tt.taskID, tt.taskID, tt.body)
			w := httptest.NewRecorder()

			handler.UpdateTask(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("UpdateTask() status = %d, want %d. Body: %s", w.Code, tt.wantStatus, w.Body.String())
			}

			// For success case, verify response contains updated status
			if tt.wantStatus == http.StatusOK {
				var result map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
					t.Fatalf("failed to parse response: %v", err)
				}
				if got := result["status"]; got != tt.body["status"] {
					t.Errorf("response status = %q, want %q", got, tt.body["status"])
				}
			}
		})
	}
}

// TestUpdateTask_EditHistoryRecordsCorrectOldValue verifies that edit_history
// records the actual old status before mutation.
// Regression test for REVIEW.md Issue #2.
func TestUpdateTask_EditHistoryRecordsCorrectOldValue(t *testing.T) {
	setupTestDB(t)

	taskID := "55555555-5555-5555-5555-555555555555"
	newStatus := "in_progress"

	var originalStatus string
	err := db.Pool.QueryRow(context.Background(),
		"SELECT status FROM tasks WHERE id = $1", taskID,
	).Scan(&originalStatus)
	if err != nil {
		t.Fatalf("failed to get original status: %v", err)
	}
	t.Cleanup(func() {
		restoreTaskStatus(t, taskID, originalStatus)
	})

	req := newAuthenticatedRequest(t, "PUT", "/api/tasks/"+taskID, taskID, map[string]string{"status": newStatus})
	w := httptest.NewRecorder()

	handler.UpdateTask(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("UpdateTask() status = %d, want 200. Body: %s", w.Code, w.Body.String())
	}

	var oldValue, newValue string
	err = db.Pool.QueryRow(context.Background(),
		`SELECT old_value, new_value FROM edit_history
		 WHERE task_id = $1 AND field_name = 'status'
		 ORDER BY edited_at DESC LIMIT 1`, taskID,
	).Scan(&oldValue, &newValue)
	if err != nil {
		t.Fatalf("failed to query edit_history: %v", err)
	}

	if oldValue != originalStatus {
		t.Errorf("edit_history.old_value = %q, want %q", oldValue, originalStatus)
	}
	if newValue != newStatus {
		t.Errorf("edit_history.new_value = %q, want %q", newValue, newStatus)
	}
	if oldValue == newValue {
		t.Error("BUG: old_value == new_value, edit history should record different values")
	}
}

// restoreTaskStatus resets a task's status to its original value after a test.
func restoreTaskStatus(t *testing.T, taskID, status string) {
	t.Helper()
	req := newAuthenticatedRequest(t, "PUT", "/api/tasks/"+taskID, taskID, map[string]string{"status": status})
	w := httptest.NewRecorder()
	handler.UpdateTask(w, req)
}
