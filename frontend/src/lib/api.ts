import { Task, DashboardStats, LoginResponse, CreateTaskRequest, UpdateTaskRequest } from '@/types';

const API_URL = process.env.NEXT_PUBLIC_API_URL || '';

class ApiClient {
  private token: string | null = null;

  setToken(token: string) {
    this.token = token;
    if (typeof window !== 'undefined') {
      localStorage.setItem('auth_token', token);
    }
  }

  getToken(): string | null {
    if (this.token) return this.token;
    if (typeof window !== 'undefined') {
      this.token = localStorage.getItem('auth_token');
    }
    return this.token;
  }

  logout() {
    this.token = null;
    if (typeof window !== 'undefined') {
      localStorage.removeItem('auth_token');
    }
  }

  private async request<T>(path: string, options: RequestInit = {}): Promise<T> {
    const token = this.getToken();
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      ...((options.headers as Record<string, string>) || {}),
    };

    if (token) {
      headers['Authorization'] = `Bearer ${token}`;
    }

    const response = await fetch(`${API_URL}${path}`, {
      ...options,
      headers,
    });

    if (!response.ok) {
      if (response.status === 401) {
        this.logout();
        throw new Error('Session expired. Please log in again.');
      }
      const error = await response.json().catch(() => ({ error: 'Unknown error' }));
      throw new Error(error.error || `HTTP ${response.status}`);
    }

    if (response.status === 204) {
      return {} as T;
    }

    return response.json();
  }

  // Auth
  async login(email: string, password: string): Promise<LoginResponse> {
    const data = await this.request<LoginResponse>('/api/auth/login', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    });
    this.setToken(data.token);
    return data;
  }

  // Tasks
  async getTasks(status?: string, include?: string): Promise<Task[]> {
    const params = new URLSearchParams();
    if (status) params.set('status', status);
    if (include) params.set('include', include);
    const query = params.toString();
    return this.request<Task[]>(`/api/tasks${query ? `?${query}` : ''}`);
  }

  async getTask(id: string): Promise<Task> {
    return this.request<Task>(`/api/tasks/${id}`);
  }

  async createTask(data: CreateTaskRequest): Promise<Task> {
    return this.request<Task>('/api/tasks', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async updateTask(id: string, data: UpdateTaskRequest): Promise<Task> {
    return this.request<Task>(`/api/tasks/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    });
  }

  async deleteTask(id: string): Promise<void> {
    await this.request<void>(`/api/tasks/${id}`, {
      method: 'DELETE',
    });
  }

  async classifyTask(id: string): Promise<Task> {
    return this.request<Task>(`/api/tasks/${id}/classify`, {
      method: 'POST',
    });
  }

  async searchTasks(query: string): Promise<Task[]> {
    return this.request<Task[]>(`/api/tasks/search?q=${encodeURIComponent(query)}`);
  }

  // Dashboard
  async getDashboardStats(): Promise<DashboardStats> {
    return this.request<DashboardStats>('/api/dashboard/stats');
  }
}

export const api = new ApiClient();
