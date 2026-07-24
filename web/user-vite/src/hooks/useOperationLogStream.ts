import { useEffect, useState } from "react";
import { getOperationLogStreamURL } from "@/api/admin/operationLogs";
import { getAdminFetchHeaders } from "@/api/admin/client";
import type { OperationLogEntry, OperationLogLevel, OperationLogScope } from "@/types/admin";

export interface UseOperationLogStreamOptions {
  scope?: OperationLogScope;
  targetId?: string;
  agentHostId?: number;
  level?: OperationLogLevel;
  enabled?: boolean;
  limit?: number;
  initialAfterId?: number;
  reconnectDelayMs?: number;
  maxEntries?: number;
}

export interface UseOperationLogStreamResult {
  logs: OperationLogEntry[];
  connected: boolean;
  error: string | null;
  lastEventId?: number;
}

type ServerSentEvent = {
  id?: number;
  event: string;
  data: string;
};

const parseEventFrame = (frame: string): ServerSentEvent | null => {
  const normalized = frame.replace(/\r\n/g, "\n").replace(/\n$/, "");
  if (!normalized) {
    return null;
  }

  let id: number | undefined;
  let event = "message";
  const dataLines: string[] = [];

  for (const line of normalized.split("\n")) {
    if (line.startsWith(":")) {
      continue;
    }
    const separatorIndex = line.indexOf(":");
    const field = separatorIndex === -1 ? line : line.slice(0, separatorIndex);
    const value = separatorIndex === -1 ? "" : line.slice(separatorIndex + 1).replace(/^ /, "");

    if (field === "id") {
      const parsedID = Number(value);
      if (Number.isFinite(parsedID)) {
        id = parsedID;
      }
    } else if (field === "event") {
      event = value;
    } else if (field === "data") {
      dataLines.push(value);
    }
  }

  return { id, event, data: dataLines.join("\n") };
};

const toErrorMessage = (error: unknown): string => {
  if (error instanceof Error) {
    return error.message;
  }
  return "Operation log stream failed";
};

export function useOperationLogStream({
  scope,
  targetId,
  agentHostId,
  level,
  enabled = true,
  limit = 100,
  initialAfterId = 0,
  maxEntries,
}: UseOperationLogStreamOptions): UseOperationLogStreamResult {
  const [logs, setLogs] = useState<OperationLogEntry[]>([]);
  const [connected, setConnected] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [lastEventId, setLastEventId] = useState<number | undefined>(
    initialAfterId > 0 ? initialAfterId : undefined
  );

  useEffect(() => {
    if (!enabled || !scope || !targetId) {
      setConnected(false);
      return;
    }

    let active = true;
    let reconnectTimer: number | undefined;
    let reconnectAttempt = 0;
    let lastID = initialAfterId;
    const abortController = new AbortController();

    const getReconnectDelay = (attempt: number) => {
      return Math.min(30000, Math.pow(2, attempt) * 3000);
    };

    setLogs([]);
    setConnected(false);
    setError(null);
    setLastEventId(initialAfterId > 0 ? initialAfterId : undefined);

    const appendLog = (entry: OperationLogEntry) => {
      lastID = Math.max(lastID, entry.id);
      setLastEventId(lastID > 0 ? lastID : undefined);
      setLogs((current) => {
        if (current.some((item) => item.id === entry.id)) {
          return current;
        }
        const next = [...current, entry].sort((a, b) => a.id - b.id);
        return maxEntries && next.length > maxEntries ? next.slice(-maxEntries) : next;
      });
    };

    const readStream = async (response: Response) => {
      const reader = response.body?.getReader();
      if (!reader) {
        throw new Error("Operation log stream is not readable");
      }

      const decoder = new TextDecoder();
      let buffer = "";

      while (active) {
        const { value, done } = await reader.read();
        if (done) {
          break;
        }
        buffer += decoder.decode(value, { stream: true }).replace(/\r\n/g, "\n");

        let separatorIndex = buffer.indexOf("\n\n");
        while (separatorIndex !== -1) {
          const frame = buffer.slice(0, separatorIndex);
          buffer = buffer.slice(separatorIndex + 2);
          const parsed = parseEventFrame(frame);
          if (parsed?.event === "operation_log" && parsed.data) {
            appendLog(JSON.parse(parsed.data) as OperationLogEntry);
          } else if (parsed?.event === "error") {
            setError(parsed.data || "Operation log stream returned an error");
          }
          separatorIndex = buffer.indexOf("\n\n");
        }
      }
    };

    const connect = async () => {
      try {
        const url = getOperationLogStreamURL({
          scope,
          target_id: targetId,
          level,
          agent_host_id: agentHostId,
          after_id: lastID > 0 ? lastID : undefined,
          limit,
        });
        const response = await fetch(url, {
          headers: getAdminFetchHeaders({ Accept: "text/event-stream" }),
          credentials: "include",
          signal: abortController.signal,
        });

        if (!response.ok) {
          throw new Error(`Operation log stream failed with status ${response.status}`);
        }

        setConnected(true);
        setError(null);
        reconnectAttempt = 0;
        await readStream(response);
        if (active && !abortController.signal.aborted) {
          setConnected(false);
          reconnectAttempt++;
          reconnectTimer = window.setTimeout(connect, getReconnectDelay(reconnectAttempt));
        }
      } catch (streamError) {
        if (!active || abortController.signal.aborted) {
          return;
        }
        setConnected(false);
        setError(toErrorMessage(streamError));
        reconnectAttempt++;
        reconnectTimer = window.setTimeout(connect, getReconnectDelay(reconnectAttempt));
      }
    };

    void connect();

    return () => {
      active = false;
      abortController.abort();
      if (reconnectTimer !== undefined) {
        window.clearTimeout(reconnectTimer);
      }
    };
  }, [agentHostId, enabled, initialAfterId, level, limit, maxEntries, scope, targetId]);

  return { logs, connected, error, lastEventId };
}
