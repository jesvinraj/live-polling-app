import React from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';
import { BarChart3, LogOut, PlusCircle, User } from 'lucide-react';

export const Navbar: React.FC = () => {
  const { user, logout } = useAuth();
  const navigate = useNavigate();

  const handleLogout = () => {
    logout();
    navigate('/login');
  };

  return (
    <header className="bg-white border-b border-gray-200 sticky top-0 z-30">
      <div className="max-w-5xl mx-auto px-4 sm:px-6 lg:px-8 h-16 flex items-center justify-between">
        <Link to="/" className="flex items-center gap-2 font-bold text-xl text-brand-600 hover:text-brand-700 transition">
          <BarChart3 className="w-6 h-6" />
          <span>LivePoll</span>
        </Link>

        <nav className="flex items-center gap-3">
          {user ? (
            <>
              <Link
                to="/dashboard"
                className="text-sm font-medium text-gray-700 hover:text-gray-900 px-3 py-2 rounded-md hover:bg-gray-100 transition flex items-center gap-1.5"
              >
                <User className="w-4 h-4 text-gray-500" />
                <span>{user.username}</span>
              </Link>
              <Link
                to="/create"
                className="text-sm font-medium bg-brand-600 text-white px-3.5 py-2 rounded-md hover:bg-brand-700 transition flex items-center gap-1.5 shadow-sm"
              >
                <PlusCircle className="w-4 h-4" />
                <span>Create Poll</span>
              </Link>
              <button
                onClick={handleLogout}
                className="text-sm font-medium text-gray-600 hover:text-red-600 p-2 rounded-md hover:bg-gray-100 transition"
                title="Logout"
              >
                <LogOut className="w-4 h-4" />
              </button>
            </>
          ) : (
            <>
              <Link
                to="/login"
                className="text-sm font-medium text-gray-700 hover:text-gray-900 px-3 py-2 rounded-md hover:bg-gray-100 transition"
              >
                Log In
              </Link>
              <Link
                to="/signup"
                className="text-sm font-medium bg-brand-600 text-white px-3.5 py-2 rounded-md hover:bg-brand-700 transition shadow-sm"
              >
                Sign Up
              </Link>
            </>
          )}
        </nav>
      </div>
    </header>
  );
};
