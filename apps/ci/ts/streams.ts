import { AnsiUp } from "ansi_up";

/**
 * Initialize stream viewers on the page. Each .stream-viewer <details> element
 * lazily loads its output on first open.
 *
 * If the run is still running, streams are fetched with ?stream=1 for live
 * tailing. Otherwise, the full content is fetched once.
 */
export function initStreamViewers(running: boolean): void {
  document.querySelectorAll<HTMLDetailsElement>(".stream-viewer").forEach((details) => {
    initOneStream(details, running);
  });
}

export function initOneStream(details: HTMLDetailsElement, running: boolean): void {
  const url = details.dataset.streamUrl;
  if (!url) return;

  const pre = details.querySelector<HTMLPreElement>(".stream-output");
  const lastLineEl = details.querySelector<HTMLSpanElement>(".stream-last-line");
  if (!pre || !lastLineEl) return;

  let loaded = false;

  // Show the last line summary from server-rendered data attribute.
  if (details.dataset.lastLine) {
    const initAnsi = new AnsiUp();
    lastLineEl.innerHTML = initAnsi.ansi_to_html(details.dataset.lastLine);
  }

  details.addEventListener("toggle", () => {
    if (!details.open || loaded) return;
    loaded = true;

    if (running) {
      streamFetch(url, pre, lastLineEl);
    } else {
      staticFetch(url, pre);
    }
  });
}

function updateLastLine(text: string, lastLineEl: HTMLSpanElement): void {
  const lines = text.split("\n");
  for (let i = lines.length - 1; i >= 0; i--) {
    if (lines[i].trim() !== "") {
      const lineAnsi = new AnsiUp();
      lastLineEl.innerHTML = lineAnsi.ansi_to_html(lines[i].substring(0, 120));
      break;
    }
  }
}

function streamFetch(url: string, pre: HTMLPreElement, lastLineEl: HTMLSpanElement): void {
  let attempt = 0;
  const baseDelay = 500;
  const maxBackoff = 30_000;

  function connect(): void {
    const ansi = new AnsiUp();
    // Clear since server replays full file on reconnect.
    pre.innerHTML = "";

    fetch(url + "?stream=1")
      .then((resp) => {
        attempt = 0;
        const reader = resp.body!.getReader();
        const decoder = new TextDecoder();
        function read(): void {
          reader
            .read()
            .then((result) => {
              if (result.done) return; // normal EOF, don't reconnect
              const text = decoder.decode(result.value, { stream: true });
              pre.innerHTML += ansi.ansi_to_html(text);
              pre.scrollTop = pre.scrollHeight;
              updateLastLine(text, lastLineEl);
              read();
            })
            .catch(() => scheduleReconnect());
        }
        read();
      })
      .catch(() => scheduleReconnect());
  }

  function scheduleReconnect(): void {
    // If the run finished while disconnected, do a static fetch instead.
    const container = document.getElementById("run-page");
    if (container?.dataset.runStatus !== "running") {
      staticFetch(url, pre);
      return;
    }
    attempt++;
    const delay = Math.min(baseDelay * 2 ** (attempt - 1), maxBackoff);
    const jitter = delay * 0.5 + Math.random() * delay * 0.5;
    setTimeout(connect, jitter);
  }

  connect();
}

function staticFetch(url: string, pre: HTMLPreElement): void {
  const ansi = new AnsiUp();
  fetch(url)
    .then((resp) => resp.text())
    .then((text) => {
      pre.innerHTML = ansi.ansi_to_html(text);
    });
}
