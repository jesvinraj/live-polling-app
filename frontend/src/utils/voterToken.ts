const VOTER_TOKEN_KEY = 'livepoll_voter_token';

function generateUUID(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID();
  }
  // Fallback RFC4122 v4 generator
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
    const r = (Math.random() * 16) | 0;
    const v = c === 'x' ? r : (r & 0x3) | 0x8;
    return v.toString(16);
  });
}

export function getOrCreateVoterToken(): string {
  let token = localStorage.getItem(VOTER_TOKEN_KEY);
  if (!token || token.length !== 36) {
    token = generateUUID();
    localStorage.setItem(VOTER_TOKEN_KEY, token);
  }
  return token;
}
