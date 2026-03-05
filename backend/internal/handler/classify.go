package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/KemenyStudio/task-manager/internal/llm"
	"github.com/KemenyStudio/task-manager/internal/model"
)

// ClassifyHandler handles AI-powered task classification.
type ClassifyHandler struct {
	llmClient llm.LLMClient
	pool      *pgxpool.Pool
}

// NewClassifyHandler creates a handler with injected dependencies.
func NewClassifyHandler(client llm.LLMClient, pool *pgxpool.Pool) *ClassifyHandler {
	return &ClassifyHandler{llmClient: client, pool: pool}
}

// Handle classifies a task using the LLM client and persists the results.
func (h *ClassifyHandler) Handle(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "id")

	// Load task from DB
	var t model.Task
	err := h.pool.QueryRow(r.Context(),
		`SELECT id, title, description, status, priority, category, summary,
		        creator_id, assignee_id, due_date, estimated_hours, actual_hours,
		        created_at, updated_at
		 FROM tasks WHERE id = $1`, taskID,
	).Scan(
		&t.ID, &t.Title, &t.Description, &t.Status, &t.Priority,
		&t.Category, &t.Summary, &t.CreatorID, &t.AssigneeID,
		&t.DueDate, &t.EstimatedHours, &t.ActualHours,
		&t.CreatedAt, &t.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		http.Error(w, `{"error": "task not found"}`, http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("classify: failed to load task %s: %v", taskID, err)
		http.Error(w, `{"error": "failed to load task"}`, http.StatusInternalServerError)
		return
	}

	if t.Title == "" {
		http.Error(w, `{"error": "task has no title, cannot classify"}`, http.StatusBadRequest)
		return
	}

	// Call LLM with 30s timeout
	llmCtx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	description := ""
	if t.Description != nil {
		description = *t.Description
	}

	classification, err := h.llmClient.ClassifyTask(llmCtx, t.Title, description)
	if err != nil {
		log.Printf("classify: LLM failed for task %s: %v", taskID, err)
		http.Error(w, `{"error": "classification failed"}`, http.StatusInternalServerError)
		return
	}

	// Persist classification results
	if err := h.persistClassification(r.Context(), taskID, classification); err != nil {
		log.Printf("classify: failed to persist for task %s: %v", taskID, err)
		http.Error(w, `{"error": "failed to save classification"}`, http.StatusInternalServerError)
		return
	}

	// Reload full task with tags, creator, assignee
	result, err := h.loadFullTask(r.Context(), taskID)
	if err != nil {
		log.Printf("classify: failed to reload task %s: %v", taskID, err)
		http.Error(w, `{"error": "classified but failed to reload task"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		log.Printf("classify: failed to encode response for task %s: %v", taskID, err)
	}
}

func (h *ClassifyHandler) persistClassification(ctx context.Context, taskID string, c *llm.TaskClassification) error {
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Update task with priority, category, and summary
	_, err = tx.Exec(ctx,
		`UPDATE tasks SET priority = $1, category = $2, summary = $3, updated_at = NOW()
		 WHERE id = $4`,
		c.Priority, c.Category, c.Summary, taskID,
	)
	if err != nil {
		return err
	}

	// Remove existing AI-assigned tags before re-classifying
	_, err = tx.Exec(ctx,
		`DELETE FROM task_tags WHERE task_id = $1 AND assigned_by = 'ai'`, taskID,
	)
	if err != nil {
		return err
	}

	// Insert tags (create if not exists) and link to task
	for _, tagName := range c.Tags {
		var tagID string
		err = tx.QueryRow(ctx,
			`INSERT INTO tags (name) VALUES ($1)
			 ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
			 RETURNING id`,
			tagName,
		).Scan(&tagID)
		if err != nil {
			return err
		}

		_, err = tx.Exec(ctx,
			`INSERT INTO task_tags (task_id, tag_id, assigned_by)
			 VALUES ($1, $2, 'ai')
			 ON CONFLICT (task_id, tag_id) DO UPDATE SET assigned_by = 'ai'`,
			taskID, tagID,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (h *ClassifyHandler) loadFullTask(ctx context.Context, taskID string) (*model.Task, error) {
	var t model.Task
	err := h.pool.QueryRow(ctx,
		`SELECT id, title, description, status, priority, category, summary,
		        creator_id, assignee_id, due_date, estimated_hours, actual_hours,
		        created_at, updated_at
		 FROM tasks WHERE id = $1`, taskID,
	).Scan(
		&t.ID, &t.Title, &t.Description, &t.Status, &t.Priority,
		&t.Category, &t.Summary, &t.CreatorID, &t.AssigneeID,
		&t.DueDate, &t.EstimatedHours, &t.ActualHours,
		&t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	// Load creator
	var creator model.User
	err = h.pool.QueryRow(ctx,
		"SELECT id, email, name, role, avatar_url, created_at, updated_at FROM users WHERE id = $1",
		t.CreatorID,
	).Scan(&creator.ID, &creator.Email, &creator.Name, &creator.Role, &creator.AvatarURL, &creator.CreatedAt, &creator.UpdatedAt)
	if err == nil {
		t.Creator = &creator
	}

	// Load assignee
	if t.AssigneeID != nil {
		var assignee model.User
		err = h.pool.QueryRow(ctx,
			"SELECT id, email, name, role, avatar_url, created_at, updated_at FROM users WHERE id = $1",
			*t.AssigneeID,
		).Scan(&assignee.ID, &assignee.Email, &assignee.Name, &assignee.Role, &assignee.AvatarURL, &assignee.CreatedAt, &assignee.UpdatedAt)
		if err == nil {
			t.Assignee = &assignee
		}
	}

	// Load tags
	tagRows, err := h.pool.Query(ctx,
		`SELECT t.id, t.name, t.color, t.created_at
		 FROM tags t
		 INNER JOIN task_tags tt ON t.id = tt.tag_id
		 WHERE tt.task_id = $1`, t.ID)
	if err == nil {
		defer tagRows.Close()
		for tagRows.Next() {
			var tag model.Tag
			if err := tagRows.Scan(&tag.ID, &tag.Name, &tag.Color, &tag.CreatedAt); err != nil {
				log.Printf("classify: failed to scan tag for task %s: %v", taskID, err)
				continue
			}
			t.Tags = append(t.Tags, tag)
		}
	}

	return &t, nil
}
