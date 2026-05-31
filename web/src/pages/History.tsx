import { useState, type FormEvent } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  getWord,
  getNeighbors,
  type WordDTO,
  type NeighborsResp,
} from "../lib/api";

// TODO: GET /api/attempts?recent=N を追加して詳細表示。
// 現状は attempts 詳細取得用 API が無いためプレースホルダ表示にとどめる。
function AttemptsPlaceholder(): JSX.Element {
  const dummyRows = [
    { id: 1, latency: "—", correct: "—", quality: "—", next_review_at: "—" },
    { id: 2, latency: "—", correct: "—", quality: "—", next_review_at: "—" },
    { id: 3, latency: "—", correct: "—", quality: "—", next_review_at: "—" },
  ];
  return (
    <section className="mb-8">
      <h2 className="text-lg font-semibold mb-2">最近の解答履歴</h2>
      <p className="text-xs text-gray-500 mb-2">
        TODO: GET /api/attempts?recent=N を追加して詳細表示。現在はプレースホルダ。
      </p>
      <div className="overflow-x-auto">
        <table className="w-full text-sm border-collapse">
          <thead>
            <tr className="bg-gray-100 text-gray-700">
              <th className="p-2 text-left">#</th>
              <th className="p-2 text-left">latency</th>
              <th className="p-2 text-left">correct</th>
              <th className="p-2 text-left">quality</th>
              <th className="p-2 text-left">next_review_at</th>
            </tr>
          </thead>
          <tbody>
            {dummyRows.map((r) => (
              <tr key={r.id} className="border-t border-gray-200">
                <td className="p-2">{r.id}</td>
                <td className="p-2">{r.latency}</td>
                <td className="p-2">{r.correct}</td>
                <td className="p-2">{r.quality}</td>
                <td className="p-2">{r.next_review_at}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}

type NetMapProps = {
  center: WordDTO;
  neighbors: NeighborsResp;
};

function NetMap({ center, neighbors }: NetMapProps): JSX.Element {
  const size = 360;
  const cx = size / 2;
  const cy = size / 2;
  const radius = 130;
  const items = neighbors.neighbors.slice(0, 5);

  const positions = items.map((n, i) => {
    const angle = (i / Math.max(items.length, 1)) * Math.PI * 2 - Math.PI / 2;
    return {
      neighbor: n,
      x: cx + radius * Math.cos(angle),
      y: cy + radius * Math.sin(angle),
    };
  });

  return (
    <div className="relative mx-auto" style={{ width: size, height: size }}>
      <svg
        className="absolute inset-0 pointer-events-none"
        width={size}
        height={size}
      >
        {positions.map((p, i) => (
          <line
            key={i}
            x1={cx}
            y1={cy}
            x2={p.x}
            y2={p.y}
            stroke="#94a3b8"
            strokeWidth={1.5}
          />
        ))}
      </svg>

      <div
        className="absolute flex items-center justify-center rounded-full bg-blue-600 text-white text-sm font-bold shadow-lg"
        style={{
          width: 90,
          height: 90,
          left: cx - 45,
          top: cy - 45,
        }}
        title={`id=${center.id}`}
      >
        {center.lemma}
      </div>

      {positions.map((p) => (
        <div
          key={p.neighbor.id}
          className="absolute flex flex-col items-center justify-center rounded-full bg-white border-2 border-emerald-400 text-xs text-gray-800 shadow"
          style={{
            width: 70,
            height: 70,
            left: p.x - 35,
            top: p.y - 35,
          }}
          title={`score=${p.neighbor.score.toFixed(3)}`}
        >
          <span className="font-semibold truncate max-w-full px-1">
            {p.neighbor.lemma || `#${p.neighbor.id}`}
          </span>
          <span className="text-[10px] text-gray-500">
            {p.neighbor.score.toFixed(2)}
          </span>
        </div>
      ))}
    </div>
  );
}

export default function History(): JSX.Element {
  const [inputId, setInputId] = useState<string>("1");
  const [queryId, setQueryId] = useState<number | null>(null);

  const wordQ = useQuery<WordDTO, Error>({
    queryKey: ["word", queryId],
    queryFn: () => getWord(queryId as number),
    enabled: queryId !== null,
  });

  const neighborsQ = useQuery<NeighborsResp, Error>({
    queryKey: ["neighbors", queryId],
    queryFn: () => getNeighbors(queryId as number, 5),
    enabled: queryId !== null,
  });

  function onSubmit(e: FormEvent<HTMLFormElement>): void {
    e.preventDefault();
    const n = parseInt(inputId, 10);
    if (!isNaN(n) && n > 0) {
      setQueryId(n);
    }
  }

  return (
    <main className="min-h-screen p-6 max-w-3xl mx-auto">
      <h1 className="text-2xl font-bold mb-6">History</h1>

      <AttemptsPlaceholder />

      <section>
        <h2 className="text-lg font-semibold mb-2">単語ネットマップ</h2>
        <form onSubmit={onSubmit} className="flex gap-2 mb-4">
          <input
            type="number"
            min={1}
            value={inputId}
            onChange={(e) => setInputId(e.target.value)}
            className="flex-1 px-3 py-2 border border-gray-300 rounded"
            placeholder="word_id"
          />
          <button
            type="submit"
            className="px-4 py-2 rounded bg-blue-600 text-white font-medium"
          >
            表示
          </button>
        </form>

        {queryId === null && (
          <p className="text-gray-500 text-sm">word_id を入力してください。</p>
        )}

        {queryId !== null && (wordQ.isLoading || neighborsQ.isLoading) && (
          <p className="text-gray-500">読み込み中...</p>
        )}

        {wordQ.error && (
          <div className="p-3 bg-red-50 border border-red-200 text-red-700 rounded mb-2">
            word: {wordQ.error.message}
          </div>
        )}
        {neighborsQ.error && (
          <div className="p-3 bg-red-50 border border-red-200 text-red-700 rounded mb-2">
            neighbors: {neighborsQ.error.message}
          </div>
        )}

        {wordQ.data && neighborsQ.data && (
          <div>
            <div className="mb-4 p-4 rounded border border-gray-200 bg-gray-50">
              <div className="text-lg font-bold">{wordQ.data.lemma}</div>
              <div className="text-xs text-gray-600 space-x-2 mt-1">
                {wordQ.data.cefr && <span>CEFR: {wordQ.data.cefr}</span>}
                {wordQ.data.pos && <span>POS: {wordQ.data.pos}</span>}
                {wordQ.data.freq_rank !== undefined && (
                  <span>freq_rank: {wordQ.data.freq_rank}</span>
                )}
              </div>
              {wordQ.data.gloss_ja && (
                <div className="text-sm text-gray-800 mt-2">
                  {wordQ.data.gloss_ja}
                </div>
              )}
            </div>
            <NetMap center={wordQ.data} neighbors={neighborsQ.data} />
          </div>
        )}
      </section>
    </main>
  );
}
