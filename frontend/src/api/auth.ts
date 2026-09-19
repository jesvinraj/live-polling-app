import { apiClient } from './client';
import { AuthResponse, User } from '../types';

export async function signup(data: { email: string; username: string; password: string }): Promise<AuthResponse> {
  return apiClient<AuthResponse>('/auth/signup', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export async function login(data: { email: string; password: string }): Promise<AuthResponse> {
  return apiClient<AuthResponse>('/auth/login', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export async function getMe(): Promise<User> {
  return apiClient<User>('/auth/me');
}
