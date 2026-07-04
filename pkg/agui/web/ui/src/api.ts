// Thin client for the AG-UI endpoints served by pkg/agui. Everything
// is same-origin (the Go server serves both this UI and the API), so
// no auth headers or base URL config is needed.
//
// The /messages endpoint already returns AG-UI-shaped messages
// (the Go handler converts stored history server-side), so there's no
// SDK→AG-UI conversion to do here — unlike the gateway demo, which
// converted on the client.

import type { Message as AGUIMessage } from "@ag-ui/core";

const API = "/api/agui";

export interface ThreadInfo {
  thread_id: string;
  conversation_id: string;
  namespace: string;
  title: string;
  message_count: number;
  created_at: string;
  updated_at: string;
}

export async function fetchAgents(): Promise<string[]> {
  const r = await fetch(`${API}/agents`);
  if (!r.ok) throw new Error(`agents → ${r.status}`);
  return (await r.json()).agents ?? [];
}

// fetchThreads returns supported=false when the agent's persistence
// adapter can't enumerate threads (the endpoint answers 501), so the
// caller can hide the conversation picker.
export async function fetchThreads(
  agent: string
): Promise<{ supported: boolean; threads: ThreadInfo[] }> {
  const r = await fetch(`${API}/agents/${encodeURIComponent(agent)}/threads`);
  if (r.status === 501) return { supported: false, threads: [] };
  if (!r.ok) throw new Error(`threads → ${r.status}`);
  return { supported: true, threads: (await r.json()).threads ?? [] };
}

export async function fetchMessages(
  agent: string,
  threadId: string
): Promise<AGUIMessage[]> {
  const r = await fetch(
    `${API}/agents/${encodeURIComponent(agent)}/threads/${encodeURIComponent(
      threadId
    )}/messages`
  );
  if (!r.ok) throw new Error(`messages → ${r.status}`);
  return (await r.json()).messages ?? [];
}

export function runUrl(agent: string): string {
  return new URL(
    `${API}/agents/${encodeURIComponent(agent)}/run`,
    window.location.origin
  ).toString();
}

export function relativeTime(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  const diff = Date.now() - d.getTime();
  const min = 60_000,
    hr = 3_600_000,
    day = 86_400_000;
  if (diff < min) return "just now";
  if (diff < hr) return `${Math.floor(diff / min)}m ago`;
  if (diff < day) return `${Math.floor(diff / hr)}h ago`;
  if (diff < 7 * day) return `${Math.floor(diff / day)}d ago`;
  return d.toLocaleDateString();
}
