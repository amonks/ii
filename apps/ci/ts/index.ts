import { initStreamViewers } from "./streams";
import { connectRunSSE } from "./sse";

function init(): void {
  const container = document.getElementById("run-page");
  if (!container) return;

  const runID = parseInt(container.dataset.runId!, 10);
  const status = container.dataset.runStatus!;
  const running = status === "running";

  // Initialize existing stream viewers.
  initStreamViewers(running);

  // Connect SSE for live updates on running builds.
  if (running) {
    connectRunSSE(runID);
  }
}

if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", init);
} else {
  init();
}
