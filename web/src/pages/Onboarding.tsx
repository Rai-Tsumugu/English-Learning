import { useState } from "react";
import { useNavigate } from "react-router-dom";
import {
  placementStart,
  placementAnswer,
  postOnboarding,
  type PlacementItemDTO,
} from "../lib/api";

type Step = "cefr" | "placement-intro" | "placement" | "submitting" | "done";

const CEFRS = ["A1", "A2", "B1", "B2", "C1", "C2"] as const;
type CEFR = (typeof CEFRS)[number];

export default function Onboarding() {
  const navigate = useNavigate();
  const [step, setStep] = useState<Step>("cefr");
  const [cefr, setCefr] = useState<CEFR | null>(null);
  const [sessionId, setSessionId] = useState<string>("");
  const [item, setItem] = useState<PlacementItemDTO | null>(null);
  const [theta, setTheta] = useState<number>(0);
  const [sem, setSem] = useState<number>(1);
  const [answered, setAnswered] = useState<number>(0);
  const [error, setError] = useState<string>("");

  async function startPlacement() {
    try {
      const r = await placementStart(0);
      setSessionId(r.session_id);
      setItem(r.item);
      setStep("placement");
    } catch (e) {
      setError(String(e));
    }
  }

  async function choose(idx: number) {
    if (!item) return;
    try {
      const r = await placementAnswer(sessionId, item.id, idx);
      setTheta(r.theta);
      setSem(r.sem);
      setAnswered((n) => n + 1);
      if (r.done) {
        await finalize(r.theta, r.sem);
      } else {
        setItem(r.item ?? null);
      }
    } catch (e) {
      setError(String(e));
    }
  }

  async function finalize(t: number, s: number) {
    if (!cefr) return;
    setStep("submitting");
    try {
      await postOnboarding({ cefr_self: cefr, theta: t, sem: s });
      setStep("done");
      setTimeout(() => navigate("/dashboard"), 800);
    } catch (e) {
      setError(String(e));
      setStep("placement-intro");
    }
  }

  async function skipPlacement() {
    await finalize(0, 1);
  }

  return (
    <main className="min-h-screen flex flex-col items-center justify-center p-6 max-w-xl mx-auto">
      <h1 className="text-2xl font-bold mb-6">Onboarding</h1>

      {error && (
        <div className="w-full mb-4 p-3 bg-red-50 border border-red-200 text-red-700 rounded">
          {error}
        </div>
      )}

      {step === "cefr" && (
        <section className="w-full space-y-4">
          <p className="text-gray-700">あなたの英語レベルを自己評価してください。</p>
          <div className="grid grid-cols-3 gap-3">
            {CEFRS.map((c) => (
              <button
                key={c}
                onClick={() => setCefr(c)}
                className={`px-4 py-3 rounded border ${
                  cefr === c ? "bg-blue-600 text-white border-blue-600" : "bg-white border-gray-300"
                }`}
              >
                {c}
              </button>
            ))}
          </div>
          <button
            disabled={!cefr}
            onClick={() => setStep("placement-intro")}
            className="w-full py-3 rounded bg-blue-600 text-white disabled:bg-gray-300"
          >
            次へ
          </button>
        </section>
      )}

      {step === "placement-intro" && (
        <section className="w-full space-y-4">
          <p className="text-gray-700">
            続いて 5〜20 問の簡単な配置テストを受けると、推奨レベルがより正確になります（スキップ可）。
          </p>
          <div className="flex gap-3">
            <button
              onClick={startPlacement}
              className="flex-1 py-3 rounded bg-blue-600 text-white"
            >
              テストを始める
            </button>
            <button onClick={skipPlacement} className="flex-1 py-3 rounded border">
              スキップ
            </button>
          </div>
        </section>
      )}

      {step === "placement" && item && (
        <section className="w-full space-y-4">
          <div className="flex justify-between text-sm text-gray-500">
            <span>第 {answered + 1} 問</span>
            <span>
              θ={theta.toFixed(2)} SEM={sem.toFixed(2)}
            </span>
          </div>
          <p className="text-lg font-medium">{item.prompt}</p>
          <div className="space-y-2">
            {item.choices.map((c, idx) => (
              <button
                key={idx}
                onClick={() => choose(idx)}
                className="w-full text-left px-4 py-3 rounded border hover:bg-gray-50"
              >
                {String.fromCharCode(65 + idx)}. {c}
              </button>
            ))}
          </div>
        </section>
      )}

      {step === "submitting" && <p>保存中...</p>}
      {step === "done" && (
        <p className="text-green-700">完了しました。ダッシュボードへ移動します...</p>
      )}
    </main>
  );
}
