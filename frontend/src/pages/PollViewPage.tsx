import React, { useEffect, useState } from 'react';
import { useParams, useNavigate, Link } from 'react-router-dom';
import { getPoll, vote as apiVote, checkVoteStatus, closePoll as apiClosePoll, deletePoll as apiDeletePoll } from '../api/polls';
import { Poll } from '../types';
import { getOrCreateVoterToken } from '../utils/voterToken';
import { usePollWebSocket } from '../hooks/usePollWebSocket';
import { ConnectionBadge } from '../components/ConnectionBadge';
import { PollResults } from '../components/PollResults';
import { QRCodeModal } from '../components/QRCodeModal';
import { DeleteConfirmModal } from '../components/DeleteConfirmModal';
import { AlertCircle, AlertTriangle, Check, CheckCircle2, Copy, Lock, QrCode, Trash2, ArrowLeft } from 'lucide-react';

export const PollViewPage: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();

  const [poll, setPoll] = useState<Poll | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');
  const [hasVoted, setHasVoted] = useState(false);
  const [userVotedOptionId, setUserVotedOptionId] = useState<string | null>(null);
  const [selectedOptionId, setSelectedOptionId] = useState<string | null>(null);
  const [isSubmittingVote, setIsSubmittingVote] = useState(false);
  const [voteError, setVoteError] = useState('');
  const [copied, setCopied] = useState(false);

  // Modals
  const [isQRModalOpen, setIsQRModalOpen] = useState(false);
  const [isDeleteModalOpen, setIsDeleteModalOpen] = useState(false);
  const [isClosingPoll, setIsClosingPoll] = useState(false);
  const [isDeletingPoll, setIsDeletingPoll] = useState(false);

  const voterToken = getOrCreateVoterToken();

  // Load initial poll data and vote status
  useEffect(() => {
    if (!id) return;

    let isMounted = true;
    const fetchPollData = async () => {
      setIsLoading(true);
      setError('');
      try {
        const [pollData, statusData] = await Promise.all([
          getPoll(id),
          checkVoteStatus(id, voterToken),
        ]);

        if (isMounted) {
          setPoll(pollData);
          setHasVoted(statusData.hasVoted);

          // Check if user voted in localStorage for option highlighting
          const savedVoteOption = localStorage.getItem(`voted_opt_${id}`);
          if (savedVoteOption) {
            setUserVotedOptionId(savedVoteOption);
          }
        }
      } catch (err: any) {
        if (isMounted) {
          setError(err.message || 'Poll not found.');
        }
      } finally {
        if (isMounted) {
          setIsLoading(false);
        }
      }
    };

    fetchPollData();
    return () => {
      isMounted = false;
    };
  }, [id, voterToken]);

  // Real-time WebSocket hook
  const initialVotesMap = React.useMemo(() => {
    const map: Record<string, number> = {};
    if (poll?.options) {
      poll.options.forEach((opt) => {
        map[opt.id] = opt.voteCount;
      });
    }
    return map;
  }, [poll]);

  const {
    votes,
    totalVotes,
    viewerCount,
    isClosed: wsIsClosed,
    isDeleted: wsIsDeleted,
    status: wsStatus,
  } = usePollWebSocket({
    pollId: id || '',
    initialVotes: initialVotesMap,
    initialTotalVotes: poll?.totalVotes || 0,
    initialIsClosed: poll?.isClosed || false,
  });

  const isPollClosed = wsIsClosed || poll?.isClosed || false;

  const handleVoteSubmit = async () => {
    if (!id || !selectedOptionId) {
      setVoteError('Please select an option to vote.');
      return;
    }

    if (isPollClosed) {
      setVoteError('This poll is closed and no longer accepting votes.');
      return;
    }

    setIsSubmittingVote(true);
    setVoteError('');

    try {
      await apiVote(id, selectedOptionId, voterToken);
      setHasVoted(true);
      setUserVotedOptionId(selectedOptionId);
      localStorage.setItem(`voted_opt_${id}`, selectedOptionId);
    } catch (err: any) {
      setVoteError(err.message || 'Failed to record vote. You may have already voted.');
      if (err.message?.includes('already voted')) {
        setHasVoted(true);
      }
    } finally {
      setIsSubmittingVote(false);
    }
  };

  const handleCopyLink = () => {
    navigator.clipboard.writeText(window.location.href);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const handleClosePoll = async () => {
    if (!id) return;
    setIsClosingPoll(true);
    try {
      await apiClosePoll(id);
      setPoll((prev) => (prev ? { ...prev, isClosed: true } : null));
    } catch (err: any) {
      alert(err.message || 'Failed to close poll.');
    } finally {
      setIsClosingPoll(false);
    }
  };

  const handleDeletePoll = async () => {
    if (!id) return;
    setIsDeletingPoll(true);
    try {
      await apiDeletePoll(id);
      navigate('/dashboard');
    } catch (err: any) {
      alert(err.message || 'Failed to delete poll.');
      setIsDeletingPoll(false);
      setIsDeleteModalOpen(false);
    }
  };

  if (isLoading) {
    return (
      <div className="min-h-[60vh] flex flex-col items-center justify-center">
        <div className="w-8 h-8 border-4 border-brand-200 border-t-brand-600 rounded-full animate-spin mb-3"></div>
        <p className="text-sm text-gray-500">Loading poll...</p>
      </div>
    );
  }

  if (error || !poll) {
    return (
      <div className="max-w-md mx-auto px-4 py-16 text-center">
        <div className="w-12 h-12 rounded-full bg-red-50 text-red-600 flex items-center justify-center mx-auto mb-4">
          <AlertCircle className="w-6 h-6" />
        </div>
        <h2 className="text-xl font-bold text-gray-900 mb-2">Poll Not Found</h2>
        <p className="text-sm text-gray-500 mb-6">
          {error || 'This poll does not exist or may have been deleted by the owner.'}
        </p>
        <Link
          to="/"
          className="inline-flex items-center gap-1.5 text-sm font-semibold text-brand-600 hover:text-brand-700 hover:underline"
        >
          <ArrowLeft className="w-4 h-4" />
          <span>Back to Home</span>
        </Link>
      </div>
    );
  }

  if (wsIsDeleted) {
    return (
      <div className="max-w-md mx-auto px-4 py-16 text-center">
        <div className="w-12 h-12 rounded-full bg-amber-50 text-amber-600 flex items-center justify-center mx-auto mb-4">
          <AlertTriangle className="w-6 h-6" />
        </div>
        <h2 className="text-xl font-bold text-gray-900 mb-2">Poll Deleted</h2>
        <p className="text-sm text-gray-500 mb-6">
          This poll was just deleted by its owner and is no longer available.
        </p>
        <Link
          to="/"
          className="inline-flex items-center gap-1.5 text-sm font-semibold text-brand-600 hover:text-brand-700 hover:underline"
        >
          <ArrowLeft className="w-4 h-4" />
          <span>Back to Home</span>
        </Link>
      </div>
    );
  }

  return (
    <div className="max-w-2xl mx-auto px-4 py-8">
      {/* Top Navigation & Status */}
      <div className="flex items-center justify-between gap-4 mb-6">
        <Link
          to="/dashboard"
          className="text-xs font-medium text-gray-500 hover:text-gray-900 flex items-center gap-1"
        >
          <ArrowLeft className="w-3.5 h-3.5" />
          <span>Dashboard</span>
        </Link>

        <ConnectionBadge status={wsStatus} viewerCount={viewerCount} />
      </div>

      {/* Main Card */}
      <div className="bg-white rounded-xl border border-gray-200 p-6 sm:p-8 shadow-sm">
        {/* Closed Banner */}
        {isPollClosed && (
          <div className="mb-6 p-3.5 bg-gray-100 border border-gray-200 text-gray-700 text-sm rounded-lg flex items-center gap-2">
            <Lock className="w-4 h-4 text-gray-500 flex-shrink-0" />
            <span className="font-medium">This poll is closed. Voting has ended.</span>
          </div>
        )}

        {/* Question Header */}
        <div className="mb-6">
          <h1 className="text-2xl font-bold text-gray-900 leading-snug">
            {poll.question}
          </h1>
          <div className="flex items-center gap-4 text-xs text-gray-400 mt-2">
            <span>
              Created {new Date(poll.createdAt).toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' })}
            </span>
            {poll.closesAt && (
              <span>
                Closes {new Date(poll.closesAt).toLocaleString(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })}
              </span>
            )}
          </div>
        </div>

        {/* Voting or Live Results View */}
        {!hasVoted && !isPollClosed ? (
          <div className="space-y-4">
            {voteError && (
              <div className="p-3.5 bg-red-50 border border-red-200 text-red-700 text-sm rounded-lg flex items-start gap-2.5">
                <AlertCircle className="w-4 h-4 flex-shrink-0 mt-0.5" />
                <span>{voteError}</span>
              </div>
            )}

            <div className="space-y-2.5">
              {poll.options.map((opt) => {
                const isSelected = selectedOptionId === opt.id;
                return (
                  <button
                    key={opt.id}
                    type="button"
                    onClick={() => setSelectedOptionId(opt.id)}
                    className={`w-full text-left p-4 rounded-lg border text-sm font-medium transition flex items-center justify-between ${
                      isSelected
                        ? 'border-brand-600 bg-brand-50/40 text-brand-900 ring-2 ring-brand-500/20'
                        : 'border-gray-200 hover:border-gray-300 hover:bg-gray-50/50 text-gray-800'
                    }`}
                  >
                    <span>{opt.text}</span>
                    <div
                      className={`w-5 h-5 rounded-full border flex items-center justify-center transition ${
                        isSelected
                          ? 'border-brand-600 bg-brand-600 text-white'
                          : 'border-gray-300 bg-white'
                      }`}
                    >
                      {isSelected && <Check className="w-3 h-3 stroke-[3]" />}
                    </div>
                  </button>
                );
              })}
            </div>

            <button
              type="button"
              disabled={!selectedOptionId || isSubmittingVote}
              onClick={handleVoteSubmit}
              className="w-full mt-2 bg-brand-600 hover:bg-brand-700 text-white font-semibold py-3 px-4 rounded-lg transition disabled:opacity-50 text-sm shadow-sm"
            >
              {isSubmittingVote ? 'Submitting Vote...' : 'Submit Vote'}
            </button>
          </div>
        ) : (
          <div>
            {hasVoted && !isPollClosed && (
              <div className="mb-5 p-3 bg-emerald-50 border border-emerald-200 text-emerald-800 text-xs font-medium rounded-lg flex items-center gap-2">
                <CheckCircle2 className="w-4 h-4 text-emerald-600 flex-shrink-0" />
                <span>Your vote was submitted. Results update live below.</span>
              </div>
            )}

            <PollResults
              options={poll.options}
              votes={votes}
              totalVotes={totalVotes}
              userVotedOptionId={userVotedOptionId}
            />
          </div>
        )}

        {/* Share & Owner Controls */}
        <div className="mt-8 pt-6 border-t border-gray-100 flex flex-col sm:flex-row sm:items-center justify-between gap-4">
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={handleCopyLink}
              className="inline-flex items-center gap-1.5 text-xs font-semibold text-gray-700 bg-gray-100 hover:bg-gray-200 px-3.5 py-2 rounded-lg transition"
            >
              {copied ? <Check className="w-3.5 h-3.5 text-emerald-600" /> : <Copy className="w-3.5 h-3.5" />}
              <span>{copied ? 'Link Copied' : 'Copy Link'}</span>
            </button>

            <button
              type="button"
              onClick={() => setIsQRModalOpen(true)}
              className="inline-flex items-center gap-1.5 text-xs font-semibold text-gray-700 bg-gray-100 hover:bg-gray-200 px-3.5 py-2 rounded-lg transition"
            >
              <QrCode className="w-3.5 h-3.5" />
              <span>QR Code</span>
            </button>
          </div>

          {poll.isOwner && (
            <div className="flex items-center gap-2">
              {!isPollClosed && (
                <button
                  type="button"
                  disabled={isClosingPoll}
                  onClick={handleClosePoll}
                  className="inline-flex items-center gap-1 text-xs font-medium text-gray-600 hover:text-gray-900 border border-gray-200 px-3 py-2 rounded-lg hover:bg-gray-50 transition"
                >
                  <Lock className="w-3.5 h-3.5" />
                  <span>{isClosingPoll ? 'Closing...' : 'Close Poll'}</span>
                </button>
              )}

              <button
                type="button"
                onClick={() => setIsDeleteModalOpen(true)}
                className="inline-flex items-center gap-1 text-xs font-medium text-red-600 hover:text-red-700 border border-red-200 bg-red-50/50 hover:bg-red-50 px-3 py-2 rounded-lg transition"
              >
                <Trash2 className="w-3.5 h-3.5" />
                <span>Delete</span>
              </button>
            </div>
          )}
        </div>
      </div>

      {/* QR Code Modal */}
      <QRCodeModal
        isOpen={isQRModalOpen}
        onClose={() => setIsQRModalOpen(false)}
        url={window.location.href}
        question={poll.question}
      />

      {/* Delete Confirmation Modal */}
      <DeleteConfirmModal
        isOpen={isDeleteModalOpen}
        onClose={() => setIsDeleteModalOpen(false)}
        onConfirm={handleDeletePoll}
        isDeleting={isDeletingPoll}
      />
    </div>
  );
};
