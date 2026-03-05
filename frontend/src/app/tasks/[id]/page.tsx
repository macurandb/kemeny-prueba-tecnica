'use client';

import { useEffect, useState } from 'react';
import { useParams } from 'next/navigation';
import { Task } from '@/types';
import { api } from '@/lib/api';

const priorityColors: Record<string, string> = {
  low: '#6b7280',
  medium: '#3b82f6',
  high: '#f59e0b',
  urgent: '#ef4444',
};

const statusColors: Record<string, string> = {
  todo: '#6b7280',
  in_progress: '#3b82f6',
  review: '#8b5cf6',
  done: '#10b981',
};

export default function TaskDetailPage() {
  const params = useParams();
  const taskId = params.id as string;

  const [task, setTask] = useState<Task | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [updateError, setUpdateError] = useState<string | null>(null);
  const [classifying, setClassifying] = useState(false);

  useEffect(() => {
    if (taskId) {
      loadTask();
    }
  }, [taskId]);

  async function loadTask() {
    try {
      setLoading(true);
      const data = await api.getTask(taskId);
      setTask(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load task');
    } finally {
      setLoading(false);
    }
  }

  async function handleClassify() {
    if (!task) return;
    setUpdateError(null);
    setClassifying(true);
    try {
      const updated = await api.classifyTask(task.id);
      setTask(updated);
    } catch (err) {
      setUpdateError(err instanceof Error ? err.message : 'Failed to classify task');
    } finally {
      setClassifying(false);
    }
  }

  async function handleStatusChange(newStatus: string) {
    if (!task) return;
    setUpdateError(null);
    try {
      const updated = await api.updateTask(task.id, { status: newStatus });
      setTask(updated);
    } catch (err) {
      setUpdateError(err instanceof Error ? err.message : 'Failed to update task');
    }
  }

  if (loading) {
    return <div style={{ textAlign: 'center', padding: '2rem' }}>Loading task...</div>;
  }

  if (error || !task) {
    return (
      <div style={{ textAlign: 'center', padding: '2rem', color: '#ef4444' }}>
        {error || 'Task not found'}
      </div>
    );
  }

  return (
    <div style={{ maxWidth: '800px' }}>
      <a href="/tasks" style={{ color: '#3b82f6', textDecoration: 'none', fontSize: '0.875rem' }}>
        ← Back to tasks
      </a>

      <div
        style={{
          backgroundColor: 'white',
          borderRadius: '0.5rem',
          padding: '2rem',
          marginTop: '1rem',
          boxShadow: '0 1px 3px rgba(0,0,0,0.1)',
        }}
      >
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
          <h1 style={{ margin: '0 0 1rem 0', fontSize: '1.5rem' }}>{task.title}</h1>
          <div style={{ display: 'flex', gap: '0.5rem' }}>
            <span
              style={{
                padding: '0.25rem 0.75rem',
                borderRadius: '9999px',
                fontSize: '0.75rem',
                fontWeight: 600,
                backgroundColor: `${priorityColors[task.priority]}20`,
                color: priorityColors[task.priority],
              }}
            >
              {task.priority}
            </span>
            <span
              style={{
                padding: '0.25rem 0.75rem',
                borderRadius: '9999px',
                fontSize: '0.75rem',
                fontWeight: 600,
                backgroundColor: `${statusColors[task.status]}20`,
                color: statusColors[task.status],
              }}
            >
              {task.status.replace('_', ' ')}
            </span>
          </div>
        </div>

        {task.description && (
          <div style={{ marginBottom: '1.5rem' }}>
            <h3 style={{ fontSize: '0.875rem', color: '#6b7280', marginBottom: '0.5rem' }}>
              Description
            </h3>
            <p style={{ margin: 0, lineHeight: 1.6, color: '#374151' }}>{task.description}</p>
          </div>
        )}

        {task.tags && task.tags.length > 0 && (
          <div style={{ marginBottom: '1.5rem' }}>
            <h3 style={{ fontSize: '0.875rem', color: '#6b7280', marginBottom: '0.5rem' }}>Tags</h3>
            <div style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap' }}>
              {task.tags.map((tag) => (
                <span
                  key={tag.id}
                  style={{
                    padding: '0.25rem 0.75rem',
                    borderRadius: '9999px',
                    fontSize: '0.75rem',
                    backgroundColor: `${tag.color}20`,
                    color: tag.color,
                    border: `1px solid ${tag.color}40`,
                  }}
                >
                  {tag.name}
                </span>
              ))}
            </div>
          </div>
        )}

        <div style={{ marginBottom: '1.5rem' }}>
          <button
            onClick={handleClassify}
            disabled={classifying}
            style={{
              padding: '0.5rem 1rem',
              borderRadius: '0.375rem',
              border: '1px solid #8b5cf6',
              backgroundColor: classifying ? '#e5e7eb' : '#8b5cf620',
              color: classifying ? '#6b7280' : '#8b5cf6',
              fontSize: '0.875rem',
              fontWeight: 600,
              cursor: classifying ? 'not-allowed' : 'pointer',
            }}
          >
            {classifying
              ? 'Classifying...'
              : task.category && task.summary
                ? 'Re-classify with AI'
                : 'Classify with AI'}
          </button>
        </div>

        <div
          style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(2, 1fr)',
            gap: '1rem',
            marginBottom: '1.5rem',
            padding: '1rem',
            backgroundColor: '#f9fafb',
            borderRadius: '0.375rem',
          }}
        >
          <div>
            <span style={{ fontSize: '0.75rem', color: '#6b7280' }}>Creator</span>
            <p style={{ margin: '0.25rem 0 0', fontWeight: 500 }}>
              {task.creator?.name || 'Unknown'}
            </p>
          </div>
          <div>
            <span style={{ fontSize: '0.75rem', color: '#6b7280' }}>Assignee</span>
            <p style={{ margin: '0.25rem 0 0', fontWeight: 500 }}>
              {task.assignee?.name || 'Unassigned'}
            </p>
          </div>
          <div>
            <span style={{ fontSize: '0.75rem', color: '#6b7280' }}>Due Date</span>
            <p style={{ margin: '0.25rem 0 0', fontWeight: 500 }}>
              {task.due_date ? new Date(task.due_date).toLocaleDateString() : 'No due date'}
            </p>
          </div>
          <div>
            <span style={{ fontSize: '0.75rem', color: '#6b7280' }}>Estimated Hours</span>
            <p style={{ margin: '0.25rem 0 0', fontWeight: 500 }}>{task.estimated_hours || '—'}</p>
          </div>
        </div>

        {updateError && (
          <div
            style={{
              padding: '0.75rem 1rem',
              marginBottom: '1rem',
              backgroundColor: '#fef2f2',
              border: '1px solid #fca5a5',
              borderRadius: '0.375rem',
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'center',
            }}
          >
            <span style={{ color: '#dc2626', fontSize: '0.875rem' }}>{updateError}</span>
            <button
              onClick={() => setUpdateError(null)}
              style={{
                background: 'none',
                border: 'none',
                color: '#dc2626',
                cursor: 'pointer',
                fontSize: '1.25rem',
                lineHeight: 1,
                padding: '0 0.25rem',
              }}
            >
              x
            </button>
          </div>
        )}

        <div>
          <h3 style={{ fontSize: '0.875rem', color: '#6b7280', marginBottom: '0.5rem' }}>
            Change Status
          </h3>
          <div style={{ display: 'flex', gap: '0.5rem' }}>
            {['todo', 'in_progress', 'review', 'done'].map((status) => (
              <button
                key={status}
                onClick={() => handleStatusChange(status)}
                disabled={task.status === status}
                style={{
                  padding: '0.5rem 1rem',
                  borderRadius: '0.375rem',
                  border: 'none',
                  cursor: task.status === status ? 'default' : 'pointer',
                  backgroundColor: task.status === status ? statusColors[status] : '#e5e7eb',
                  color: task.status === status ? 'white' : '#374151',
                  fontSize: '0.875rem',
                  opacity: task.status === status ? 1 : 0.7,
                }}
              >
                {status.replace('_', ' ')}
              </button>
            ))}
          </div>
        </div>

        {task.summary && (
          <div
            style={{
              marginTop: '1.5rem',
              padding: '1rem',
              backgroundColor: '#eff6ff',
              borderRadius: '0.375rem',
            }}
          >
            <h3
              style={{
                fontSize: '0.875rem',
                color: '#1d4ed8',
                marginBottom: '0.25rem',
                marginTop: 0,
              }}
            >
              AI Summary
            </h3>
            <p style={{ margin: 0, color: '#1e40af', fontSize: '0.875rem' }}>{task.summary}</p>
          </div>
        )}
      </div>
    </div>
  );
}
