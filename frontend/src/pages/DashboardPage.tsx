import React, { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { getMyPolls } from '../api/polls';
import { PollSummary } from '../types';
import { PollCard } from '../components/PollCard';
import { AlertCircle, BarChart3, PlusCircle, RefreshCw } from 'lucide-react';

export const DashboardPage: React.FC = () => {
  const [polls, setPolls] = useState<PollSummary[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');

  const fetchPolls = async () => {
    setIsLoading(true);
    setError('');
    try {
      const data = await getMyPolls();
      setPolls(data);
    } catch (err: any) {
      setError(err.message || 'Failed to load your polls.');
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    fetchPolls();
  }, []);

  return (
    <div className="max-w-5xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 mb-8">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Your Polls</h1>
          <p className="text-sm text-gray-500 mt-1">Manage and view the live results of your polls</p>
        </div>

        <div className="flex items-center gap-3">
          <button
            onClick={fetchPolls}
            disabled={isLoading}
            className="p-2 text-gray-600 hover:text-gray-900 border border-gray-200 rounded-lg hover:bg-gray-100 transition disabled:opacity-50"
            title="Refresh list"
          >
            <RefreshCw className={`w-4 h-4 ${isLoading ? 'animate-spin' : ''}`} />
          </button>
          <Link
            to="/create"
            className="bg-brand-600 hover:bg-brand-700 text-white font-semibold text-sm px-4 py-2 rounded-lg transition flex items-center gap-2 shadow-sm"
          >
            <PlusCircle className="w-4 h-4" />
            <span>Create Poll</span>
          </Link>
        </div>
      </div>

      {error && (
        <div className="mb-6 p-4 bg-red-50 border border-red-200 text-red-700 text-sm rounded-lg flex items-start gap-3">
          <AlertCircle className="w-5 h-5 flex-shrink-0 mt-0.5" />
          <div>
            <p className="font-semibold">Error loading polls</p>
            <p className="mt-0.5">{error}</p>
          </div>
        </div>
      )}

      {isLoading ? (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {[1, 2, 3, 4].map((n) => (
            <div key={n} className="bg-white border border-gray-200 rounded-lg p-5 h-36 animate-pulse">
              <div className="h-4 bg-gray-200 rounded w-1/4 mb-3"></div>
              <div className="h-5 bg-gray-200 rounded w-3/4 mb-4"></div>
              <div className="h-4 bg-gray-100 rounded w-1/2"></div>
            </div>
          ))}
        </div>
      ) : polls.length === 0 ? (
        <div className="bg-white border border-gray-200 border-dashed rounded-xl p-12 text-center">
          <div className="w-12 h-12 rounded-full bg-brand-50 text-brand-600 flex items-center justify-center mx-auto mb-4">
            <BarChart3 className="w-6 h-6" />
          </div>
          <h3 className="text-lg font-bold text-gray-900 mb-1">No polls created yet</h3>
          <p className="text-sm text-gray-500 max-w-sm mx-auto mb-6">
            Create your first live poll to share with your audience and see votes stream in instantly.
          </p>
          <Link
            to="/create"
            className="inline-flex items-center gap-2 bg-brand-600 hover:bg-brand-700 text-white font-semibold text-sm px-4 py-2.5 rounded-lg transition shadow-sm"
          >
            <PlusCircle className="w-4 h-4" />
            <span>Create Your First Poll</span>
          </Link>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {polls.map((poll) => (
            <PollCard key={poll.id} poll={poll} />
          ))}
        </div>
      )}
    </div>
  );
};
