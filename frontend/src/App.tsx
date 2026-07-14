import { AlertCircle, ArrowRight, Check, Copy, Loader2, RefreshCw } from "lucide-react";
import type { TFunction } from "i18next";
import type { ReactNode } from "react";
import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  ApiError,
  AttemptResult,
  Locale,
  QuizQuestion,
  SubmitAnswerResponse,
  TodayQuiz,
  loadAttempt,
  loadTodayQuiz,
  startTodayAttempt,
  submitAnswer,
} from "./api/client";
import { isSupportedLocale, supportedLocales } from "./i18n";

type AnswerMap = Record<string, SubmitAnswerResponse>;
type AnswerState = "correct" | "incorrect" | "pending";

export function App() {
  const { t, i18n } = useTranslation();
  const [locale, setLocale] = useState<Locale>(currentLocale(i18n.language));
  const [quiz, setQuiz] = useState<TodayQuiz | null>(null);
  const [attemptId, setAttemptId] = useState<string | null>(null);
  const [attemptResult, setAttemptResult] = useState<AttemptResult | null>(null);
  const [answers, setAnswers] = useState<AnswerMap>({});
  const [currentQuestionIndex, setCurrentQuestionIndex] = useState(0);
  const [selectedLanguageId, setSelectedLanguageId] = useState<string | null>(null);
  const [questionStartedAt, setQuestionStartedAt] = useState(Date.now());
  const [showResult, setShowResult] = useState(false);
  const [isLoading, setIsLoading] = useState(true);
  const [isBusy, setIsBusy] = useState(false);
  const [errorCode, setErrorCode] = useState<string | null>(null);

  useEffect(() => {
    void i18n.changeLanguage(locale);
    window.localStorage.setItem("discerne_locale", locale);
  }, [i18n, locale]);

  useEffect(() => {
    let active = true;

    async function startAttempt(nextQuiz: TodayQuiz) {
      const started = await startTodayAttempt();
      const result = await loadAttempt(started.attemptId);
      if (!active) {
        return;
      }

      setAttemptId(result.attemptId);
      setAttemptResult(result);
      setAnswers(answerMapFromAnswers(result.answers));
      setSelectedLanguageId(null);
      setQuestionStartedAt(Date.now());
      setShowResult(result.status === "completed");
      setCurrentQuestionIndex(nextQuestionIndex(result, nextQuiz.questions.length));
    }

    async function loadQuiz() {
      setIsLoading(true);
      setErrorCode(null);

      try {
        const nextQuiz = await loadTodayQuiz(locale);
        if (!active) {
          return;
        }

        const sameQuizDate = quiz?.quizDate === nextQuiz.quizDate;
        const existingAttempt = attemptResultFromTodayQuiz(nextQuiz);
        setQuiz(nextQuiz);
        if (!sameQuizDate) {
          setAnswers({});
          setSelectedLanguageId(null);
          setQuestionStartedAt(Date.now());
          setShowResult(false);
        }

        if (existingAttempt === null) {
          setAttemptId(null);
          setAttemptResult(null);
          await startAttempt(nextQuiz);
          return;
        }

        setAttemptId(existingAttempt.attemptId);
        setAttemptResult(existingAttempt);
        setAnswers(answerMapFromAnswers(existingAttempt.answers));
        setShowResult((current) => {
          if (sameQuizDate) {
            return current;
          }
          return existingAttempt.status === "completed";
        });
        setCurrentQuestionIndex((current) => {
          if (sameQuizDate) {
            return Math.min(current, Math.max(nextQuiz.questions.length - 1, 0));
          }
          return nextQuestionIndex(existingAttempt, nextQuiz.questions.length);
        });
      } catch (error) {
        if (!active) {
          return;
        }
        setErrorCode(errorCodeFromError(error));
        setQuiz(null);
        setAttemptId(null);
        setAttemptResult(null);
        setAnswers({});
      } finally {
        if (active) {
          setIsLoading(false);
        }
      }
    }

    void loadQuiz();

    return () => {
      active = false;
    };
  }, [locale]);

  const currentQuestion = quiz?.questions[currentQuestionIndex] ?? null;
  const currentAnswer = currentQuestion === null ? null : answers[currentQuestion.id] ?? null;
  const completed = attemptResult?.status === "completed";
  const formattedDate = useMemo(() => {
    if (quiz === null) {
      return t("quiz.today");
    }
    return formatQuizDate(quiz.quizDate, locale);
  }, [locale, quiz, t]);

  async function handleLocaleChange(nextLocale: Locale) {
    setLocale(nextLocale);
  }

  async function handleSubmitAnswer(languageId: string) {
    if (attemptId === null || currentQuestion === null || currentAnswer !== null || isBusy) {
      return;
    }

    setSelectedLanguageId(languageId);
    setIsBusy(true);
    setErrorCode(null);

    try {
      const responseTimeMs = Math.max(0, Date.now() - questionStartedAt);
      const answer = await submitAnswer(attemptId, currentQuestion.id, languageId, responseTimeMs);
      setAnswers((previous) => ({
        ...previous,
        [answer.questionId]: answer,
      }));
      const result = await loadAttempt(attemptId);
      setAttemptResult(result);
      setAnswers(answerMapFromAnswers(result.answers));
    } catch (error) {
      setErrorCode(errorCodeFromError(error));
    } finally {
      setIsBusy(false);
    }
  }

  function handleNextQuestion() {
    if (quiz === null) {
      return;
    }
    if (currentQuestion?.position === quiz.questions.length && attemptResult?.status === "completed") {
      setShowResult(true);
      return;
    }
    setSelectedLanguageId(null);
    setCurrentQuestionIndex((current) => Math.min(current + 1, quiz.questions.length - 1));
    setQuestionStartedAt(Date.now());
  }

  function handleRefresh() {
    setAnswers({});
    setSelectedLanguageId(null);
    setQuestionStartedAt(Date.now());
    setShowResult(false);
    setLocale((current) => current);
    window.location.reload();
  }

  return (
    <main className="app-shell">
      <header className="top-bar">
        <div className="date-block">
          <div className="date-line">
            {t("quiz.dateLabel")}: {formattedDate}
          </div>
        </div>

        <div className="title-block">
          <div className="brand">{t("app.brand")}</div>
        </div>

        <div className="locale-switcher" aria-label="Locale">
          {supportedLocales.map((item) => (
            <button
              className={item === locale ? "locale-button locale-button--active" : "locale-button"}
              key={item}
              onClick={() => void handleLocaleChange(item)}
              type="button"
            >
              {t(`locale.${item}`)}
            </button>
          ))}
        </div>
      </header>

      {isLoading ? (
        <StatusPanel icon={<Loader2 className="spin" size={22} />} text={t("quiz.loading")} />
      ) : errorCode !== null && quiz === null ? (
        <StatusPanel icon={<AlertCircle size={22} />} text={messageForError(errorCode, t)}>
          <button className="primary-button" onClick={handleRefresh} type="button">
            <RefreshCw size={18} />
            {t("quiz.restart")}
          </button>
        </StatusPanel>
      ) : quiz === null ? (
        <StatusPanel icon={<AlertCircle size={22} />} text={t("quiz.noQuiz")} />
      ) : completed && showResult && attemptResult !== null ? (
        <ResultPanel quizDate={quiz.quizDate} result={attemptResult} total={quiz.questions.length} />
      ) : attemptId === null ? (
        <StatusPanel icon={<Loader2 className="spin" size={22} />} text={t("quiz.loading")} />
      ) : currentQuestion !== null ? (
        <QuestionPanel
          answer={currentAnswer}
          busy={isBusy}
          errorCode={errorCode}
          onAnswer={(languageId) => void handleSubmitAnswer(languageId)}
          onNext={handleNextQuestion}
          question={currentQuestion}
          questionAnswerStates={questionAnswerStates(quiz.questions, answers)}
          questionCount={quiz.questions.length}
          selectedLanguageId={selectedLanguageId}
        />
      ) : null}
    </main>
  );
}

type QuestionPanelProps = {
  answer: SubmitAnswerResponse | null;
  busy: boolean;
  errorCode: string | null;
  onAnswer: (languageId: string) => void;
  onNext: () => void;
  question: QuizQuestion;
  questionAnswerStates: AnswerState[];
  questionCount: number;
  selectedLanguageId: string | null;
};

function QuestionPanel({
  answer,
  busy,
  errorCode,
  onAnswer,
  onNext,
  question,
  questionAnswerStates,
  questionCount,
  selectedLanguageId,
}: QuestionPanelProps) {
  const { t } = useTranslation();
  const isAnswered = answer !== null;
  const correctOption = answer === null ? null : question.options.find((option) => option.languageId === answer.correctLanguageId);
  const isLastQuestion = question.position >= questionCount;

  return (
    <section className="quiz-panel">
      <div className="panel-header">
        <StatusBadge>{t("quiz.questionProgress", { current: question.position, total: questionCount })}</StatusBadge>
        <QuestionProgressDots states={questionAnswerStates} />
      </div>

      <h1 className="question-title">{t("quiz.prompt")}</h1>

      <p className="quiz-text" dir={textDirection(question.text)}>
        {question.text}
      </p>

      <div className="option-grid" aria-label={t("quiz.choose")}>
        {question.options.map((option) => {
          const selected = selectedLanguageId === option.languageId || answer?.selectedLanguageId === option.languageId;
          const correct = answer?.correctLanguageId === option.languageId;
          const wrongSelection = selected && isAnswered && !correct;
          const className = [
            "option-button",
            selected ? "option-button--selected" : "",
            correct && isAnswered ? "option-button--correct" : "",
            wrongSelection ? "option-button--incorrect" : "",
          ]
            .filter(Boolean)
            .join(" ");

          return (
            <button
              aria-pressed={selected}
              className={className}
              disabled={busy || isAnswered}
              key={option.languageId}
              onClick={() => onAnswer(option.languageId)}
              type="button"
            >
              <span>{option.name}</span>
            </button>
          );
        })}
      </div>

      {answer !== null ? (
        <div className={answer.isCorrect ? "feedback feedback--correct" : "feedback feedback--incorrect"}>
          <div className="feedback-title">{answer.isCorrect ? t("quiz.correct") : t("quiz.incorrect")}</div>
          {!answer.isCorrect && correctOption !== undefined && correctOption !== null ? (
            <div>{t("quiz.correctAnswer", { name: correctOption.name })}</div>
          ) : null}
        </div>
      ) : null}

      {errorCode !== null ? (
        <div className="error-line">
          <AlertCircle size={18} />
          {messageForError(errorCode, t)}
        </div>
      ) : null}

      {answer !== null ? (
        <div className="action-row">
          <button className="primary-button" onClick={onNext} type="button">
            <ArrowRight size={18} />
            {isLastQuestion ? t("quiz.finish") : t("quiz.next")}
          </button>
        </div>
      ) : null}

      <div className="progress-track" aria-hidden="true">
        <span style={{ width: `${Math.max(8, (question.position / questionCount) * 100)}%` }} />
      </div>
    </section>
  );
}

function QuestionProgressDots({ states }: { states: AnswerState[] }) {
  const { t } = useTranslation();

  return (
    <div className="question-progress-frame" aria-label={t("quiz.questionResults")}>
      {states.map((state, index) => (
        <span
          aria-label={labelForAnswerState(state, t)}
          className={`question-progress-dot question-progress-dot--${state}`}
          key={index}
          role="img"
        />
      ))}
    </div>
  );
}

function ResultPanel({ quizDate, result, total }: { quizDate: string; result: AttemptResult; total: number }) {
  const { t } = useTranslation();
  const [copied, setCopied] = useState(false);
  const score = result.score ?? 0;
  const answerResults = Array.from({ length: total }, (_, index) => result.answers[index]?.isCorrect ?? false);
  const shareLine = answerResults.map((isCorrect) => (isCorrect ? "🟢" : "🔴")).join("");
  const shareText = `Discerne! ${quizDate}\n${shareLine}`;

  async function handleCopyResult() {
    try {
      await copyTextToClipboard(shareText);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1800);
    } catch {
      setCopied(false);
    }
  }

  return (
    <section className="quiz-panel result-panel">
      <StatusBadge>{t("quiz.completed")}</StatusBadge>
      <h1>{t("quiz.resultTitle")}</h1>
      <div className="score-value">{score} / {total}</div>
      <div className="result-dots" aria-label={t("quiz.questionResults")}>
        {answerResults.map((isCorrect, index) => (
          <span
            aria-label={isCorrect ? t("quiz.correct") : t("quiz.incorrect")}
            className={isCorrect ? "result-dot result-dot--correct" : "result-dot result-dot--incorrect"}
            key={index}
            role="img"
          />
        ))}
      </div>
      <button className="primary-button copy-result-button" onClick={() => void handleCopyResult()} type="button">
        {copied ? <Check size={18} /> : <Copy size={18} />}
        {copied ? t("quiz.resultCopied") : t("quiz.copyResult")}
      </button>
    </section>
  );
}

function StatusPanel({ children, icon, text }: { children?: ReactNode; icon: ReactNode; text: string }) {
  return (
    <section className="status-panel">
      {icon}
      <p>{text}</p>
      {children}
    </section>
  );
}

function StatusBadge({ children }: { children: ReactNode }) {
  return <span className="status-badge">{children}</span>;
}

function currentLocale(value: string): Locale {
  if (isSupportedLocale(value)) {
    return value;
  }
  return "en-US";
}

function attemptResultFromTodayQuiz(quiz: TodayQuiz): AttemptResult | null {
  const status = quiz.attempt.status;
  if (status === "not_started" || quiz.attempt.attemptId === undefined) {
    return null;
  }

  return {
    attemptId: quiz.attempt.attemptId,
    status,
    answeredCount: quiz.attempt.answeredCount,
    questionCount: quiz.attempt.questionCount,
    score: quiz.attempt.score,
    answers: quiz.attempt.answers,
  };
}

function answerMapFromAnswers(answers: SubmitAnswerResponse[]) {
  return answers.reduce<AnswerMap>((result, answer) => {
    result[answer.questionId] = answer;
    return result;
  }, {});
}

function questionAnswerStates(questions: QuizQuestion[], answers: AnswerMap): AnswerState[] {
  return questions.map((question) => {
    const answer = answers[question.id];
    if (answer === undefined) {
      return "pending";
    }
    return answer.isCorrect ? "correct" : "incorrect";
  });
}

function nextQuestionIndex(result: AttemptResult, questionCount: number) {
  return Math.min(Math.max(result.answeredCount, 0), Math.max(questionCount - 1, 0));
}

function formatQuizDate(quizDate: string, locale: Locale) {
  return new Intl.DateTimeFormat(locale, { dateStyle: "long" }).format(new Date(`${quizDate}T12:00:00`));
}

function textDirection(text: string): "ltr" | "rtl" {
  return /[\u0590-\u05ff\u0600-\u06ff]/.test(text) ? "rtl" : "ltr";
}

async function copyTextToClipboard(text: string) {
  if (navigator.clipboard !== undefined) {
    await navigator.clipboard.writeText(text);
    return;
  }

  const textArea = document.createElement("textarea");
  textArea.value = text;
  textArea.style.position = "fixed";
  textArea.style.left = "-9999px";
  textArea.style.top = "0";
  document.body.appendChild(textArea);
  textArea.focus();
  textArea.select();

  try {
    document.execCommand("copy");
  } finally {
    document.body.removeChild(textArea);
  }
}

function errorCodeFromError(error: unknown) {
  if (error instanceof ApiError) {
    return error.code;
  }
  return "network";
}

function messageForError(code: string, t: TFunction) {
  const key = `errors.${code}`;
  const message = t(key);
  return message === key ? t("errors.default") : message;
}

function labelForAnswerState(state: AnswerState, t: TFunction) {
  switch (state) {
    case "correct":
      return t("quiz.correct");
    case "incorrect":
      return t("quiz.incorrect");
    case "pending":
      return t("quiz.unanswered");
  }
}
