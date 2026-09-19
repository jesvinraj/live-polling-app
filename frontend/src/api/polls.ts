import { apiClient } from './client';
import { Poll, PollSummary, VoteResult } from '../types';

export async function createPoll(data: {
  question: string;
  options: string[];
  closesAt?: string;
}): Promise<Poll> {
  return apiClient<Poll>('/polls', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export async function getMyPolls(): Promise<PollSummary[]> {
  return apiClient<PollSummary[]>('/polls/my');
}

export async function getPoll(id: string): Promise<Poll> {
  return apiClient<Poll>(`/polls/${id}`);
}

export async function vote(
  pollId: string,
  optionId: string,
  voterToken: string
): Promise<VoteResult> {
  return apiClient<VoteResult>(`/polls/${pollId}/vote`, {
    method: 'POST',
    body: JSON.stringify({ optionId, voterToken }),
  });
}

export async function checkVoteStatus(
  pollId: string,
  voterToken: string
): Promise<{ hasVoted: boolean }> {
  return apiClient<{ hasVoted: boolean }>(
    `/polls/${pollId}/status?voterToken=${encodeURIComponent(voterToken)}`
  );
}

export async function closePoll(id: string): Promise<Poll> {
  return apiClient<Poll>(`/polls/${id}/close`, {
    method: 'PATCH',
  });
}

export async function deletePoll(id: string): Promise<{ message: string }> {
  return apiClient<{ message: string }>(`/polls/${id}`, {
    method: 'DELETE',
  });
}
