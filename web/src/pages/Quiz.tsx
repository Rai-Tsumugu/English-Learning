import { useEffect, useMemo, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useSSE } from "../hooks/useSSE";
import { postAttempt, type GenItem } from "../lib/api";

type Phase = "loading" | "quizzing" | "finished" | "error";

function getUserId(): number {
  try {
    const raw = localStorage.getItem("user_id");
    if (!raw) return 1;
    const n = parseInt(raw, 10);
    return Number.isFinite(n) && n > 0 ? n : 1;
  } catch {
    return 1;
  }
}

async function sha256Hex(s: string): Promise<string> {
  const enc = new TextEncoder().encode(s);
  const buf = await crypto.subtle.digest("SHA-256", enc);
  return Array.from(new Uint8Array(buf))
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");
}

export default function Quiz(): JSX.Element {
  const navigate = useNavigate();
  const userId = useMemo(() => getUserId(), []);
  const url = `/api/sessions/today?user_id=${userId}&cefr=A2`;

  const [phase, setPhase] = useState<Phase>("loading");
  const [planTotal, setPlanTotal] = useState<number | null>(null);
  const [queue, setQueue] = useState<GenItem[]>([]);
  const [idx, setIdx] = useState(0);
  const [selected, setSelected] = useState<number | null>(null);
  const [judged, setJudged] = useState<null | "correct" | "wrong">(null);
  const [flash, setFlash] = useState(false);
  const [serverErr, setServerErr] = useState<string | null>(null);
  const [wrongStreak, setWrongStreak] = useState(0);
  const [showHint, setShowHint] = useState(false);
  const [streamDone, setStreamDone] = useState(false);
  const startedAtRef = useRef<number>(Date.now());

  const handlers = useMemo(
    () => ({
      plan: (data: string) => {
        try {
          const j = JSON.parse(data) as { total?: number };
          if (typeof j.total === "number") setPlanTotal(j.total);
        } catch {
          /* noop */
        }
        setPhase("quizzing");
      },
      question: (data: string) => {
        try {
          const item = JSON.parse(data) as GenItem;
          setQueue((q) => [...q, item]);
        } catch {
          /* skip */
        }
      },
      done: () => {
        setStreamDone(true);
      },
      error: (data: string) => {
        setServerErr(data || "server error");
        setPhase("error");
      },
    }),
    [],
  );

  const { lastError } = useSSE(url, handlers);

  const current: GenItem | undefined = queue[idx];

  // reset per-question state
  useEffect(() => {
    setSelected(null);
    setJudged(null);
    setShowHint(false);
    startedAtRef.current = Date.now();
  }, [idx]);

  // 全問終了判定: ストリームが done かつ idx が queue を超えた
  useEffect(() => {
    if (streamDone && queue.length > 0 && idx >= queue.length) {
      setPhase("finished");
    }
  }, [streamDone, queue.length, idx]);

  async function onAnswer(choiceIdx: number) {
    if (!current || judged) return;
    setSelected(choiceIdx);
    const correct = choiceIdx === current.answer_index;
    const latency = Date.now() - startedAtRef.current;
    const hash = await sha256Hex(
      `${current.target_lemma}|${current.prompt}|${current.choices.join("|")}`,
    );
    setJudged(correct ? "correct" : "wrong");
    if (correct) {
      setFlash(true);
      setWrongStreak(0);
      window.setTimeout(() => setFlash(false), 500);
    } else {
      setWrongStreak((n) => n + 1);
    }
    try {
      await postAttempt({
        user_id: userId,
        word_id: 0,
        content_hash: hash,
        correct,
        latency_ms: latency,
      });
    } catch (e) {
      setServerErr(e instanceof Error ? e.message : String(e));
    }
  }

  function onNext() {
    setIdx((i) => i + 1);
  }

  if (phase === "error") {
    return (
      <main className="min-h-screen p-6 max-w-2xl mx-auto">
        <h1 className="text-2xl font-bold mb-4">Quiz</h1>
        <div className="p-3 bg-red-50 border border-red-200 text-red-700 rounded">
          {serverErr ?? lastError ?? "エラーが発生しました"}
        </div>
        <button
          onClick={() => navigate("/dashboard")}
          className="mt-6 px-4 py-2 bg-gray-200 rounded"
        >
          ダッシュボードに戻る
        </button>
      </main>
    );
  }

  if (phase === "finished") {
    return (
      <main className="min-h-screen flex flex-col items-center justify-center gap-6 p-6">
        <h1 className="text-3xl font-bold">お疲れさまでした</h1>
        <p className="text-gray-600">{queue.length} 問完了しました。</p>
        <button
          onClick={() => navigate("/dashboard")}
          className="px-6 py-3 bg-blue-700 hover:bg-blue-800 text-white font-bold rounded-lg"
        >
          ダッシュボードに戻る
        </button>
      </main>
    );
  }

  return (
    <main
      className={
        "min-h-screen p-6 max-w-2xl mx-auto transition-colors duration-300 " +
        (flash ? "bg-emerald-100" : "")
      }
    >
      <h1 className="text-2xl font-bold mb-2">Quiz</h1>
      <p className="text-sm text-gray-600 mb-6">
        {planTotal !== null ? `今日の ${planTotal} 問` : "問題を準備中..."}
        {current && planTotal !== null
          ? ` (${Math.min(idx + 1, planTotal)} / ${planTotal})`
          : null}
      </p>

      {!current && (
        <div className="text-gray-500">問題を取得しています...</div>
      )}

      {current && (
        <section className="space-y-4">
          <div className="text-xs uppercase text-gray-500">
            {current.question_type} · {current.cefr_evidence}
          </div>
          <h2 className="text-xl font-semibold leading-relaxed">
            {current.prompt}
          </h2>

          <ul className="space-y-2">
            {current.choices.map((c, i) => {
              const isSel = selected === i;
              const isAns = current.answer_index === i;
              let cls =
                "w-full text-left px-4 py-3 rounded border transition-colors ";
              if (judged) {
                if (isAns) cls += "bg-emerald-100 border-emerald-400 ";
                else if (isSel) cls += "bg-red-100 border-red-400 ";
                else cls += "bg-white border-gray-200 ";
              } else {
                cls +=
                  "bg-white border-gray-200 hover:bg-gray-50 cursor-pointer ";
              }
              return (
                <li key={i}>
                  <button
                    disabled={judged !== null}
                    className={cls}
                    onClick={() => onAnswer(i)}
                  >
                    <span className="inline-block w-6 font-mono text-gray-500">
                      {String.fromCharCode(65 + i)}.
                    </span>
                    {c}
                    {judged && isAns && (
                      <span className="ml-2 text-emerald-600 font-bold">
                        ✓
                      </span>
                    )}
                  </button>
                </li>
              );
            })}
          </ul>

          {judged === "wrong" && wrongStreak >= 3 && !showHint && (
            <button
              onClick={() => setShowHint(true)}
              className="px-3 py-2 bg-yellow-100 border border-yellow-300 rounded text-sm"
            >
              ヒントを見る (リカバリーチケット)
            </button>
          )}
          {showHint && (
            <div className="p-3 bg-yellow-50 border border-yellow-300 rounded text-sm">
              <div className="font-semibold mb-1">答えのヒント:</div>
              <div>{current.answer_span}</div>
            </div>
          )}

          {judged && (
            <div className="pt-2">
              <button
                onClick={onNext}
                className="px-5 py-2 bg-blue-700 hover:bg-blue-800 text-white font-bold rounded"
              >
                {idx + 1 < queue.length || !streamDone ? "次へ" : "結果を見る"}
              </button>
            </div>
          )}

          {serverErr && (
            <div className="p-2 bg-red-50 border border-red-200 text-red-700 rounded text-sm">
              {serverErr}
            </div>
          )}
        </section>
      )}
    </main>
  );
}
