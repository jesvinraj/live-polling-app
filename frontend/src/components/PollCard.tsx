import React from 'react';
import { Link } from 'react-router-dom';
import { PollSummary } from '../types';
import { BarChart2, CheckCircle2, Clock } from 'lucide-react';

interface PollCardProps {
  poll: PollSummary;
}

export const PollCard: React.FC<PollCardProps> = ({ poll }) => {
  const formattedDate = new Date(poll.createdAt).toLocaleDateString(undefined, {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  });

  return (
    <div className="bg-white rounded-lg border border-gray-200 p-5 hover:border-gray-300 transition shadow-sm flex flex-col justify-between">
      <div>
        <div className="flex items-center justify-between gap-2 mb-2">
          <span
            className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${
              poll.isClosed
                ? 'bg-gray-100 text-gray-700'
                : 'bg-emerald-50 text-emerald-700 border border-emerald-200'
            }`}
          >
            {poll.isClosed ? 'Closed' : 'Active'}
          </span>
          <span className="text-xs text-gray-500">{formattedDate}</span>
        </div>

        <h3 className="font-semibold text-gray-900 text-lg line-clamp-2 mb-3">
          {poll.question}
        </h3>
      </div>

      <div className="pt-4 border-t border-gray-100 flex items-center justify-between">
        <div className="flex items-center gap-4 text-xs text-gray-600">
          <span className="flex items-center gap-1">
            <CheckCircle2 className="w-3.5 h-3.5 text-gray-400" />
            {poll.totalVotes} {poll.totalVotes === 1 ? 'vote' : 'votes'}
          </span>
          <span className="flex items-center gap-1">
            <BarChart2 className="w-3.5 h-3.5 text-gray-400" />
            {poll.optionCount} options
          </span>
          {poll.closesAt && !poll.isClosed && (
            <span className="flex items-center gap-1 text-amber-700">
              <Clock className="w-3.5 h-3.5" />
              Expires {new Date(poll.closesAt).toLocaleDateString()}
            </span>
          )}
        </div>

        <Link
          to={`/polls/${poll.id}`}
          className="text-sm font-medium text-brand-600 hover:text-brand-700 hover:underline"
        >
          View Poll
        </Link>
      </div>
    </div>
  );
};
