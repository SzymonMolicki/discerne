export type Locale = "pl-PL" | "en-US" | "es-ES";

export type QuizOption = {
  languageId: string;
  name: string;
};

export type QuizQuestion = {
  id: string;
  position: number;
  text: string;
  options: QuizOption[];
};

export type TodayQuiz = {
  quizDate: string;
  attempt: {
    status: string;
  };
  questions: QuizQuestion[];
};

export type StartAttemptResponse = {
  attemptId: string;
  status: "in_progress";
};

export type SubmitAnswerResponse = {
  questionId: string;
  selectedLanguageId: string;
  correctLanguageId: string;
  isCorrect: boolean;
};

export type AttemptResult = {
  attemptId: string;
  status: "in_progress" | "completed";
  answeredCount: number;
  questionCount: number;
  score: number | null;
};

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;

  constructor(status: number, code: string) {
    super(code);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
  }
}

const apiBaseURL = import.meta.env.VITE_API_BASE_URL ?? "";

export async function loadTodayQuiz(locale: Locale): Promise<TodayQuiz> {
  return request<TodayQuiz>(`/api/v1/quizzes/today?locale=${encodeURIComponent(locale)}`);
}

export async function startTodayAttempt(): Promise<StartAttemptResponse> {
  return request<StartAttemptResponse>("/api/v1/quizzes/today/attempt", {
    method: "POST",
  });
}

export async function submitAnswer(
  attemptId: string,
  questionId: string,
  selectedLanguageId: string,
  responseTimeMs: number,
): Promise<SubmitAnswerResponse> {
  return request<SubmitAnswerResponse>(`/api/v1/attempts/${attemptId}/answers`, {
    method: "POST",
    body: JSON.stringify({
      questionId,
      selectedLanguageId,
      responseTimeMs,
    }),
  });
}

export async function loadAttempt(attemptId: string): Promise<AttemptResult> {
  return request<AttemptResult>(`/api/v1/attempts/${attemptId}`);
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  if (init.body !== undefined && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }

  const response = await fetch(`${apiBaseURL}${path}`, {
    ...init,
    headers,
    credentials: "include",
  });

  const payload = await readJSON(response);
  if (!response.ok) {
    const code = payload && typeof payload.error === "string" ? payload.error : "request_failed";
    throw new ApiError(response.status, code);
  }

  return payload as T;
}

async function readJSON(response: Response): Promise<Record<string, unknown>> {
  const text = await response.text();
  if (text === "") {
    return {};
  }

  try {
    return JSON.parse(text) as Record<string, unknown>;
  } catch {
    if (!response.ok) {
      return { error: "invalid_response" };
    }
    throw new ApiError(response.status, "invalid_response");
  }
}
