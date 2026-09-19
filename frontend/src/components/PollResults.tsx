import React from 'react';
import { PollOption } from '../types';
import { Check } from 'lucide-react';

interface PollResultsProps {
  options: PollOption[];
  votes: Record<string, number>;
  totalVotes: number;
  userVotedOptionId?: string | null;
}

export const PollResults: React.FC<PollResultsProps> = ({
  options,
  votes,
  totalVotes,
  userVotedOptionId,
}) => {
  // Find highest vote count to highlight the leading option
  let maxVotes = 0;
  options.forEach((opt) => {
    const count = votes[opt.id] ?? opt.voteCount ?? 0;
    if (count > maxVotes) maxVotes = count;
  });

  return (
    <div className="space-y-3.5">
      {options.map((opt) => {
        const count = votes[opt.id] ?? opt.voteCount ?? 0;
        const percentage = totalVotes > 0 ? Math.round((count / totalVotes) * 100) : 0;
        const isUserChoice = userVotedOptionId === opt.id;
        const isLeading = totalVotes > 0 && count === maxVotes && maxVotes > 0;

        return (
          <div
            key={opt.id}
            className={`relative overflow-hidden rounded-lg border p-4 transition ${
              isUserChoice
                ? 'border-brand-500 bg-brand-50/20'
                : 'border-gray-200 bg-white'
            }`}
          >
            {/* Background percentage fill bar */}
            <div
              className={`absolute top-0 bottom-0 left-0 poll-bar-transition ${
                isLeading
                  ? 'bg-brand-100/70'
                  : 'bg-gray-100/80'
              }`}
              style={{ width: `${percentage}%` }}
            />

            {/* Option text and vote count display */}
            <div className="relative z-10 flex items-center justify-between gap-4">
              <div className="flex items-center gap-2">
                <span className="font-medium text-gray-900 text-base">
                  {opt.text}
                </span>
                {isUserChoice && (
                  <span className="inline-flex items-center gap-0.5 text-xs font-semibold text-brand-700 bg-brand-100 px-2 py-0.5 rounded-full">
                    <Check className="w-3 h-3" />
                    Your vote
                  </span>
                )}
              </div>

              <div className="flex items-center gap-3 text-sm flex-shrink-0">
                <span className="text-gray-500 text-xs">
                  {count} {count === 1 ? 'vote' : 'votes'}
                </span>
                <span className="font-bold text-gray-900 w-11 text-right">
                  {percentage}%
                </span>
              </div>
            </div>
          </div>
        );
      })}

      <div className="pt-2 text-right text-xs text-gray-500 font-medium">
        Total votes: {totalVotes}
      </div>
    </div>
  );
};
