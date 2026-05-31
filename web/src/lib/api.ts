export const API_BASE = "/api";

async function req<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    headers: { "Content-Type": "application/json", ...(init?.headers ?? {}) },
    ...init,
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(`${res.status} ${res.statusText}: ${text}`);
  }
  return res.json() as Promise<T>;
}

// ---- Placement ----
export type PlacementItemDTO = {
  id: string;
  prompt: string;
  choices: string[];
  cefr: string;
};

export type StartResp = {
  session_id: string;
  item: PlacementItemDTO;
};

export type AnswerResp = {
  done: boolean;
  theta: number;
  sem: number;
  cefr?: string;
  item?: PlacementItemDTO;
};

export function placementStart(userId = 0): Promise<StartResp> {
  return req<StartResp>("/placement/start", {
    method: "POST",
    body: JSON.stringify({ user_id: userId }),
  });
}

export function placementAnswer(
  sessionId: string,
  itemId: string,
  choice: number,
): Promise<AnswerResp> {
  return req<AnswerResp>("/placement/answer", {
    method: "POST",
    body: JSON.stringify({ session_id: sessionId, item_id: itemId, choice }),
  });
}

// ---- Onboarding ----
export type OnboardingResp = {
  user_id: number;
  cefr_self: string;
  theta: number;
  sem: number;
};

export function postOnboarding(input: {
  cefr_self: string;
  theta?: number;
  sem?: number;
}): Promise<OnboardingResp> {
  return req<OnboardingResp>("/onboarding", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

// ---- Stats ----
export type WeeklyStatsResp = {
  remaining_words: number;
  cache_hit_rate: number;
  total_attempts_7d: number;
  streak_days: number;
  est_cost_usd_7d: number;
};

export function getWeeklyStats(): Promise<WeeklyStatsResp> {
  return req<WeeklyStatsResp>("/stats/weekly");
}

// ---- Words ----
export type WordDTO = {
  id: number;
  lemma: string;
  cefr?: string;
  freq_rank?: number;
  pos?: string;
  gloss_ja?: string;
};

export type NeighborHit = {
  id: number;
  lemma: string;
  score: number;
};

export type NeighborsResp = {
  word_id: number;
  neighbors: NeighborHit[];
};

export function getWord(id: number): Promise<WordDTO> {
  return req<WordDTO>(`/words/${id}`);
}

export function getNeighbors(id: number, k = 5): Promise<NeighborsResp> {
  return req<NeighborsResp>(`/words/${id}/neighbors?k=${k}`);
}

// ---- Sessions / Attempts ----
export type GenItem = {
  question_type: string;
  prompt: string;
  choices: string[];
  answer_index: number;
  answer_span: string;
  cefr_evidence: string;
  target_lemma: string;
};

export type AttemptReq = {
  user_id: number;
  word_id: number;
  content_hash: string;
  correct: boolean;
  latency_ms: number;
};

export type AttemptResp = {
  attempt_id: number;
  quality: number;
  next_review_at: string;
  ease: number;
  interval_days: number;
  reps: number;
};

export function postAttempt(req_: AttemptReq): Promise<AttemptResp> {
  return req<AttemptResp>("/attempts", {
    method: "POST",
    body: JSON.stringify(req_),
  });
}
