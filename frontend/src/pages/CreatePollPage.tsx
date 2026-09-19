import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { createPoll } from '../api/polls';
import { AlertCircle, Clock, Plus, Trash2 } from 'lucide-react';

export const CreatePollPage: React.FC = () => {
  const [question, setQuestion] = useState('');
  const [options, setOptions] = useState<string[]>(['', '']);
  const [closesAt, setClosesAt] = useState('');
  const [error, setError] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);

  const navigate = useNavigate();

  const handleAddOption = () => {
    if (options.length >= 10) {
      setError('You can add at most 10 options.');
      return;
    }
    setOptions([...options, '']);
  };

  const handleRemoveOption = (index: number) => {
    if (options.length <= 2) {
      setError('A poll must have at least 2 options.');
      return;
    }
    const updated = options.filter((_, i) => i !== index);
    setOptions(updated);
  };

  const handleOptionChange = (index: number, value: string) => {
    const updated = [...options];
    updated[index] = value;
    setOptions(updated);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');

    const cleanQuestion = question.trim();
    if (cleanQuestion.length < 5) {
      setError('Poll question must be at least 5 characters long.');
      return;
    }

    const cleanOptions = options.map((opt) => opt.trim()).filter(Boolean);
    if (cleanOptions.length < 2) {
      setError('Please provide at least 2 non-empty options.');
      return;
    }

    // Check duplicate options
    const set = new Set(cleanOptions.map((o) => o.toLowerCase()));
    if (set.size !== cleanOptions.length) {
      setError('Duplicate options are not allowed.');
      return;
    }

    let isoClosingDate: string | undefined = undefined;
    if (closesAt) {
      const selectedTime = new Date(closesAt);
      if (selectedTime <= new Date()) {
        setError('Closing time must be in the future.');
        return;
      }
      isoClosingDate = selectedTime.toISOString();
    }

    setIsSubmitting(true);
    try {
      const poll = await createPoll({
        question: cleanQuestion,
        options: cleanOptions,
        closesAt: isoClosingDate,
      });
      navigate(`/polls/${poll.id}`);
    } catch (err: any) {
      setError(err.message || 'Failed to create poll.');
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div className="max-w-2xl mx-auto px-4 py-8">
      <div className="bg-white rounded-xl border border-gray-200 p-6 sm:p-8 shadow-sm">
        <div className="mb-6">
          <h1 className="text-2xl font-bold text-gray-900">Create a New Poll</h1>
          <p className="text-sm text-gray-500 mt-1">
            Ask a question and share the real-time link with your audience
          </p>
        </div>

        {error && (
          <div className="mb-6 p-4 bg-red-50 border border-red-200 text-red-700 text-sm rounded-lg flex items-start gap-2.5">
            <AlertCircle className="w-5 h-5 flex-shrink-0 mt-0.5" />
            <span>{error}</span>
          </div>
        )}

        <form onSubmit={handleSubmit} className="space-y-6">
          <div>
            <label className="block text-sm font-semibold text-gray-900 mb-1.5">
              Poll Question <span className="text-red-500">*</span>
            </label>
            <input
              type="text"
              required
              maxLength={250}
              value={question}
              onChange={(e) => setQuestion(e.target.value)}
              placeholder="e.g. Which programming language do you use most?"
              className="w-full px-3.5 py-2.5 bg-white border border-gray-300 rounded-lg text-sm text-gray-900 focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500 transition"
            />
            <p className="text-xs text-gray-400 mt-1">{question.length}/250 characters</p>
          </div>

          <div>
            <div className="flex items-center justify-between mb-2">
              <label className="block text-sm font-semibold text-gray-900">
                Options <span className="text-red-500">*</span> (min 2, max 10)
              </label>
              <span className="text-xs text-gray-500">{options.length}/10</span>
            </div>

            <div className="space-y-2.5">
              {options.map((option, idx) => (
                <div key={idx} className="flex items-center gap-2">
                  <span className="text-xs font-semibold text-gray-400 w-5 text-right flex-shrink-0">
                    {idx + 1}.
                  </span>
                  <input
                    type="text"
                    required
                    maxLength={100}
                    value={option}
                    onChange={(e) => handleOptionChange(idx, e.target.value)}
                    placeholder={`Option ${idx + 1}`}
                    className="flex-1 px-3.5 py-2 bg-white border border-gray-300 rounded-lg text-sm text-gray-900 focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500 transition"
                  />
                  {options.length > 2 && (
                    <button
                      type="button"
                      onClick={() => handleRemoveOption(idx)}
                      className="text-gray-400 hover:text-red-600 p-2 rounded-lg hover:bg-gray-100 transition"
                      title="Remove option"
                    >
                      <Trash2 className="w-4 h-4" />
                    </button>
                  )}
                </div>
              ))}
            </div>

            {options.length < 10 && (
              <button
                type="button"
                onClick={handleAddOption}
                className="mt-3 inline-flex items-center gap-1.5 text-xs font-semibold text-brand-600 hover:text-brand-700 hover:bg-brand-50 px-3 py-1.5 rounded-md transition"
              >
                <Plus className="w-3.5 h-3.5" />
                <span>Add Option</span>
              </button>
            )}
          </div>

          <div className="pt-2 border-t border-gray-100">
            <label className="block text-sm font-semibold text-gray-900 mb-1 flex items-center gap-1.5">
              <Clock className="w-4 h-4 text-gray-400" />
              <span>Optional Closing Time</span>
            </label>
            <p className="text-xs text-gray-500 mb-2">
              The poll will automatically stop accepting votes after this date and time.
            </p>
            <input
              type="datetime-local"
              value={closesAt}
              onChange={(e) => setClosesAt(e.target.value)}
              className="px-3.5 py-2 bg-white border border-gray-300 rounded-lg text-sm text-gray-900 focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500 transition"
            />
          </div>

          <div className="pt-4 flex items-center justify-end gap-3">
            <button
              type="button"
              onClick={() => navigate('/dashboard')}
              className="px-4 py-2.5 text-sm font-medium text-gray-700 bg-gray-100 hover:bg-gray-200 rounded-lg transition"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={isSubmitting}
              className="bg-brand-600 hover:bg-brand-700 text-white font-semibold text-sm px-6 py-2.5 rounded-lg transition shadow-sm disabled:opacity-50"
            >
              {isSubmitting ? 'Creating poll...' : 'Create and Launch Poll'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};
