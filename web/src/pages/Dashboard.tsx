import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { getWeeklyStats, type WeeklyStatsResp } from "../lib/api";

function StreakSquares({ streakDays }: { streakDays: number }): JSX.Element {
  // 7マス。右端を「今日」とし、過去6日 → 今日 の順に並べる。
  // streak_days は「今日から遡って連続で attempts があった日数」なので、
  // 右端から streakDays 個を着色する。
  const cells: JSX.Element[] = [];
  for (let i = 0; i < 7; i++) {
    // i=0 が一番左 (6日前)、i=6 が今日
    const daysAgo = 6 - i;
    const isToday = daysAgo === 0;
    const active = daysAgo < streakDays;
    const base = "w-10 h-10 rounded flex items-center justify-center text-xs font-medium";
    const color = active
      ? "bg-emerald-500 text-white"
      : "bg-gray-200 text-gray-500";
    const ring = isToday ? " ring-4 ring-blue-400" : "";
    cells.push(
      <div key={i} className={base + " " + color + ring} title={isToday ? "今日" : `${daysAgo}日前`}>
        {isToday ? "今" : ""}
      </div>,
    );
  }
  return <div className="flex gap-2 justify-center">{cells}</div>;
}

function StatCard({ label, value }: { label: string; value: string }): JSX.Element {
  return (
    <div className="flex flex-col items-center justify-center p-5 rounded-lg border border-gray-200 bg-white shadow-sm">
      <span className="text-xs text-gray-500 mb-1">{label}</span>
      <span className="text-2xl font-bold text-gray-900">{value}</span>
    </div>
  );
}

function Skeleton(): JSX.Element {
  return (
    <div className="space-y-6 animate-pulse">
      <div className="h-20 bg-gray-200 rounded" />
      <div className="flex gap-2 justify-center">
        {Array.from({ length: 7 }).map((_, i) => (
          <div key={i} className="w-10 h-10 bg-gray-200 rounded" />
        ))}
      </div>
      <div className="grid grid-cols-3 gap-3">
        {Array.from({ length: 3 }).map((_, i) => (
          <div key={i} className="h-20 bg-gray-200 rounded" />
        ))}
      </div>
    </div>
  );
}

export default function Dashboard(): JSX.Element {
  const navigate = useNavigate();
  const { data, isLoading, error } = useQuery<WeeklyStatsResp, Error>({
    queryKey: ["weekly-stats"],
    queryFn: getWeeklyStats,
  });

  return (
    <main className="min-h-screen p-6 max-w-2xl mx-auto">
      <h1 className="text-2xl font-bold mb-6">Dashboard</h1>

      <button
        onClick={() => navigate("/quiz")}
        className="w-full mb-8 rounded-lg bg-blue-700 hover:bg-blue-800 text-white font-bold text-xl shadow-md transition-colors"
        style={{ minHeight: "72px" }}
      >
        今日の5分を始める
      </button>

      {isLoading && <Skeleton />}
      {error && (
        <div className="p-3 bg-red-50 border border-red-200 text-red-700 rounded">
          {error.message}
        </div>
      )}
      {data && (
        <div className="space-y-6">
          <section>
            <h2 className="text-sm font-medium text-gray-700 mb-2">ストリーク (直近7日)</h2>
            <StreakSquares streakDays={data.streak_days} />
            <p className="text-center text-xs text-gray-500 mt-2">
              連続 {data.streak_days} 日
            </p>
          </section>

          <section className="grid grid-cols-3 gap-3">
            <StatCard label="残語数" value={String(data.remaining_words)} />
            <StatCard
              label="7日間の学習回数"
              value={String(data.total_attempts_7d)}
            />
            <StatCard
              label="キャッシュHit率"
              value={`${(data.cache_hit_rate * 100).toFixed(1)}%`}
            />
          </section>
        </div>
      )}
    </main>
  );
}
