import { Routes, Route, Link } from "react-router-dom";
import Onboarding from "./pages/Onboarding";
import Dashboard from "./pages/Dashboard";
import History from "./pages/History";
import Quiz from "./pages/Quiz";

function Home() {
  return (
    <main className="min-h-screen flex flex-col items-center justify-center gap-4 p-8">
      <h1 className="text-3xl font-bold">English Learning — Phase1</h1>
      <p className="text-gray-600">Sprint1 縦串疎通版。</p>
      <nav className="flex gap-4">
        <Link className="text-blue-600 underline" to="/onboarding">Onboarding</Link>
        <Link className="text-blue-600 underline" to="/dashboard">Dashboard</Link>
        <Link className="text-blue-600 underline" to="/quiz">Quiz</Link>
        <Link className="text-blue-600 underline" to="/history">History</Link>
      </nav>
    </main>
  );
}

export default function App() {
  return (
    <Routes>
      <Route path="/" element={<Home />} />
      <Route path="/onboarding" element={<Onboarding />} />
      <Route path="/dashboard" element={<Dashboard />} />
      <Route path="/quiz" element={<Quiz />} />
      <Route path="/history" element={<History />} />
    </Routes>
  );
}
