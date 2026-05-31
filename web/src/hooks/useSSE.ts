import { useEffect, useRef, useState } from "react";

export type SSEHandlers = {
  plan?: (data: string) => void;
  question?: (data: string) => void;
  done?: (data: string) => void;
  error?: (data: string) => void;
};

export type UseSSEResult = {
  connected: boolean;
  lastError: string | null;
  close: () => void;
};

/**
 * Minimal wrapper around native EventSource.
 * - url=null では接続しない
 * - 名前付きイベント (plan/question/done/error) を addEventListener で受信
 * - handlers は ref で保持して再接続を防ぐ
 */
export function useSSE(url: string | null, handlers: SSEHandlers): UseSSEResult {
  const [connected, setConnected] = useState(false);
  const [lastError, setLastError] = useState<string | null>(null);
  const esRef = useRef<EventSource | null>(null);
  const handlersRef = useRef<SSEHandlers>(handlers);

  // keep latest handlers without re-subscribing
  useEffect(() => {
    handlersRef.current = handlers;
  }, [handlers]);

  useEffect(() => {
    if (!url) {
      return;
    }
    const es = new EventSource(url);
    esRef.current = es;

    const onOpen = () => setConnected(true);
    const onPlan = (e: MessageEvent) => handlersRef.current.plan?.(e.data);
    const onQuestion = (e: MessageEvent) =>
      handlersRef.current.question?.(e.data);
    const onDone = (e: MessageEvent) => {
      handlersRef.current.done?.(e.data);
      es.close();
      setConnected(false);
    };
    const onNamedError = (e: MessageEvent) => {
      handlersRef.current.error?.(e.data);
      setLastError(e.data ?? "error");
    };
    const onConnError = () => {
      setLastError("connection error");
      setConnected(false);
    };

    es.addEventListener("open", onOpen);
    es.addEventListener("plan", onPlan as EventListener);
    es.addEventListener("question", onQuestion as EventListener);
    es.addEventListener("done", onDone as EventListener);
    es.addEventListener("error", onNamedError as EventListener);
    es.onerror = onConnError;

    return () => {
      es.removeEventListener("open", onOpen);
      es.removeEventListener("plan", onPlan as EventListener);
      es.removeEventListener("question", onQuestion as EventListener);
      es.removeEventListener("done", onDone as EventListener);
      es.removeEventListener("error", onNamedError as EventListener);
      es.close();
      esRef.current = null;
      setConnected(false);
    };
  }, [url]);

  const close = () => {
    esRef.current?.close();
    esRef.current = null;
    setConnected(false);
  };

  return { connected, lastError, close };
}
