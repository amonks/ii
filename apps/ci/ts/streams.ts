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

function streamFetch(url: string, pre: HTMLPreElement, lastLineEl: HTMLSpanElement): void {
  const ansi = new AnsiUp();
  fetch(url + "?stream=1").then((resp) => {
    const reader = resp.body!.getReader();
    const decoder = new TextDecoder();
    function read(): void {
      reader.read().then((result) => {
        if (result.done) return;
        const text = decoder.decode(result.value, { stream: true });
        pre.innerHTML += ansi.ansi_to_html(text);
        pre.scrollTop = pre.scrollHeight;
        const lines = text.split("\n");
        for (let i = lines.length - 1; i >= 0; i--) {
          if (lines[i].trim() !== "") {
            const lineAnsi = new AnsiUp();
            lastLineEl.innerHTML = lineAnsi.ansi_to_html(lines[i].substring(0, 120));
            break;
          }
        }
        read();
      });
    }
    read();
  });
}

function staticFetch(url: string, pre: HTMLPreElement): void {
  const ansi = new AnsiUp();
  fetch(url)
    .then((resp) => resp.text())
    .then((text) => {
      pre.innerHTML = ansi.ansi_to_html(text);
    });
}
