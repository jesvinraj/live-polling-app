import React, { createContext, useContext, useState, useEffect } from 'react';
import { User } from '../types';
import { getMe, login as apiLogin, signup as apiSignup } from '../api/auth';
import { setOnUnauthorizedCallback } from '../api/client';

interface AuthContextType {
  user: User | null;
  token: string | null;
  isLoading: boolean;
  login: (email: string, password: string) => Promise<void>;
  signup: (email: string, username: string, password: string) => Promise<void>;
  logout: () => void;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export const AuthProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [user, setUser] = useState<User | null>(null);
  const [token, setToken] = useState<string | null>(() => localStorage.getItem('poll_token'));
  const [isLoading, setIsLoading] = useState<boolean>(true);

  const logout = () => {
    localStorage.removeItem('poll_token');
    setToken(null);
    setUser(null);
  };

  useEffect(() => {
    setOnUnauthorizedCallback(() => {
      logout();
    });

    const initAuth = async () => {
      if (token) {
        try {
          const me = await getMe();
          setUser(me);
        } catch {
          logout();
        }
      }
      setIsLoading(false);
    };

    initAuth();
  }, [token]);

  const login = async (email: string, password: string) => {
    const res = await apiLogin({ email, password });
    localStorage.setItem('poll_token', res.token);
    setToken(res.token);
    setUser(res.user);
  };

  const signup = async (email: string, username: string, password: string) => {
    const res = await apiSignup({ email, username, password });
    localStorage.setItem('poll_token', res.token);
    setToken(res.token);
    setUser(res.user);
  };

  return (
    <AuthContext.Provider value={{ user, token, isLoading, login, signup, logout }}>
      {children}
    </AuthContext.Provider>
  );
};

export const useAuth = () => {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
};
