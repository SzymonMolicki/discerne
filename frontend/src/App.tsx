import { AlertCircle, ArrowRight, Loader2, RefreshCw } from "lucide-react";
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

    async function loadQuiz() {
      setIsLoading(true);
      setErrorCode(null);

      async function startAttempt(nextQuiz: TodayQuiz) {
        const started = await startTodayAttempt();
        window.localStorage.setItem(attemptStorageKey(nextQuiz.quizDate), started.attemptId);
        const result = await loadAttempt(started.attemptId);
        if (!active) {
          return;
        }
        setAttemptId(started.attemptId);
        setAttemptResult(result);
        setShowResult(false);
        setCurrentQuestionIndex(nextQuestionIndex(result, nextQuiz.questions.length));
      }

      try {
        const nextQuiz = await loadTodayQuiz(locale);
        if (!active) {
          return;
        }

        const sameQuizDate = quiz?.quizDate === nextQuiz.quizDate;
        setQuiz(nextQuiz);
        if (!sameQuizDate) {
          setAnswers({});
          setSelectedLanguageId(null);
          setQuestionStartedAt(Date.now());
          setShowResult(false);
        }

        const storedAttemptId = window.localStorage.getItem(attemptStorageKey(nextQuiz.quizDate));
        if (storedAttemptId === null) {
          await startAttempt(nextQuiz);
          return;
        }

        try {
          const result = await loadAttempt(storedAttemptId);
          if (!active) {
            return;
          }
          setAttemptId(storedAttemptId);
          setAttemptResult(result);
          if (!sameQuizDate) {
            setShowResult(result.status === "completed");
          }
          setCurrentQuestionIndex((current) => {
            if (sameQuizDate) {
              return Math.min(current, Math.max(nextQuiz.questions.length - 1, 0));
            }
            return nextQuestionIndex(result, nextQuiz.questions.length);
          });
        } catch (error) {
          if (error instanceof ApiError && error.code === "attempt_not_found") {
            window.localStorage.removeItem(attemptStorageKey(nextQuiz.quizDate));
            await startAttempt(nextQuiz);
            return;
          }
          throw error;
        }
      } catch (error) {
        if (!active) {
          return;
        }
        setErrorCode(errorCodeFromError(error));
        setQuiz(null);
        setAttemptId(null);
        setAttemptResult(null);
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
      setAttemptResult(await loadAttempt(attemptId));
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
        <ResultPanel result={attemptResult} total={quiz.questions.length} />
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

function ResultPanel({ result, total }: { result: AttemptResult; total: number }) {
  const { t } = useTranslation();
  const score = result.score ?? 0;

  return (
    <section className="quiz-panel result-panel">
      <StatusBadge>{t("quiz.completed")}</StatusBadge>
      <h1>{t("quiz.resultTitle")}</h1>
      <div className="score-value">{score} / {total}</div>
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

function attemptStorageKey(quizDate: string) {
  return `discerne_attempt_${quizDate}`;
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
