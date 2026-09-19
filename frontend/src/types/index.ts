export interface User {
  id: string;
  email: string;
  username: string;
  createdAt: string;
}

export interface AuthResponse {
  token: string;
  user: User;
}

export interface PollOption {
  id: string;
  text: string;
  voteCount: number;
}

export interface Poll {
  id: string;
  question: string;
  options: PollOption[];
  totalVotes: number;
  isClosed: boolean;
  closesAt?: string;
  createdAt: string;
  updatedAt: string;
  isOwner: boolean;
}

export interface PollSummary {
  id: string;
  question: string;
  optionCount: number;
  totalVotes: number;
  isClosed: boolean;
  closesAt?: string;
  createdAt: string;
}

export interface VoteResult {
  pollId: string;
  optionId: string;
  votes: Record<string, number>;
  totalVotes: number;
  isClosed: boolean;
}

export type WSMessageType =
  | 'INIT_STATE'
  | 'VOTE_UPDATE'
  | 'VIEWER_UPDATE'
  | 'POLL_CLOSED'
  | 'POLL_DELETED'
  | 'ERROR';

export interface WSMessage {
  type: WSMessageType;
  pollId?: string;
  votes?: Record<string, number>;
  totalVotes?: number;
  viewerCount?: number;
  isClosed?: boolean;
  isDeleted?: boolean;
  optionId?: string;
  error?: string;
}
