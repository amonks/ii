import { SSEEvent } from "./types";

export type SSEHandler = (event: SSEEvent) => void;

export function connectSSE(mapID: number, handler: SSEHandler): EventSource {
  const path = window.location.pathname.replace(/\/$/, "");
  const es = new EventSource(`${path}/events/`);

  es.onmessage = (e) => {
    try {
      const event: SSEEvent = JSON.parse(e.data);
      handler(event);
    } catch {
      // ignore parse errors
    }
  };

  es.onerror = () => {
    // EventSource auto-reconnects
  };

  return es;
}
