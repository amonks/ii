import { initOneStream } from "./streams";

export interface StreamInfo {
  name: string;
  display_name: string;
  status: string;
  duration_ms?: number;
  error?: string;
}

export interface RunState {
  run: {
    id: number;
    status: string;
    head_sha: string;
    base_sha: string;
    trigger: string;
    started_at: string;
    finished_at?: string;
    machine_id?: string;
    error?: string;
  };
  jobs: Array<{
    name: string;
    kind: string;
    status: string;
    duration_ms?: number;
    error?: string;
  }>;
  streams: Record<string, StreamInfo[]>;
}

/**
 * Connect to the SSE endpoint for a run and update the page when events arrive.
 */
export function connectRunSSE(runID: number): EventSource {
  const path = window.location.pathname.replace(/\/$/, "");
  const es = new EventSource(`${path}/events`);

  es.onmessage = (e) => {
    try {
      const state: RunState = JSON.parse(e.data);
      applyRunState(state);
    } catch {
      // ignore parse errors
    }
  };

  es.onerror = () => {
    // EventSource auto-reconnects
  };

  return es;
}

export function applyRunState(state: RunState): void {
  updateRunMetadata(state);
  updateJobsTable(state);
}

function updateRunMetadata(state: RunState): void {
  const statusEl = document.getElementById("run-status");
  if (statusEl) {
    statusEl.textContent = state.run.status;
    statusEl.className = `status-${state.run.status}`;
  }

  const finishedEl = document.getElementById("run-finished");
  if (finishedEl) {
    if (state.run.finished_at) {
      finishedEl.textContent = state.run.finished_at;
      finishedEl.style.display = "";
    }
  }

  const errorEl = document.getElementById("run-error");
  if (errorEl) {
    if (state.run.error) {
      errorEl.textContent = state.run.error;
      errorEl.style.display = "";
    }
  }

  // Update data-run-status on container so new stream viewers know the mode.
  const container = document.getElementById("run-page");
  if (container) {
    container.dataset.runStatus = state.run.status;
  }
}

function statusClass(status: string): string {
  return `status-${status}`;
}

function updateJobsTable(state: RunState): void {
  const tbody = document.getElementById("jobs-tbody");
  if (!tbody) return;

  const noJobsEl = document.getElementById("no-jobs");
  if (noJobsEl && state.jobs.length > 0) {
    noJobsEl.style.display = "none";
  }

  // Show the jobs table if it was hidden.
  const jobsTable = document.getElementById("jobs-table");
  if (jobsTable && state.jobs.length > 0) {
    jobsTable.style.display = "";
  }

  // Build a set of existing job rows for diffing.
  const existingRows = new Map<string, HTMLTableRowElement>();
  tbody.querySelectorAll<HTMLTableRowElement>("tr[data-job-name]").forEach((row) => {
    existingRows.set(row.dataset.jobName!, row);
  });

  // Track which stream detail elements already exist.
  const existingStreams = new Set<string>();
  tbody.querySelectorAll<HTMLDetailsElement>(".stream-viewer").forEach((details) => {
    existingStreams.add(details.dataset.streamUrl || "");
  });

  // We need to rebuild the tbody to maintain ordering.
  // Collect fragments per job then replace tbody contents.
  const fragment = document.createDocumentFragment();

  const running = state.run.status === "running";

  for (const job of state.jobs) {
    // Main job row.
    const existingRow = existingRows.get(job.name);
    let row: HTMLTableRowElement;
    if (existingRow) {
      row = existingRow;
      // Update cells in place.
      const cells = row.querySelectorAll("td");
      cells[1].textContent = job.status;
      cells[1].className = statusClass(job.status);
      cells[2].textContent = job.duration_ms != null ? `${job.duration_ms}ms` : "";
    } else {
      row = document.createElement("tr");
      row.dataset.jobName = job.name;
      row.innerHTML = `
        <td>${escapeHtml(job.name)}</td>
        <td class="${statusClass(job.status)}">${escapeHtml(job.status)}</td>
        <td>${job.duration_ms != null ? `${job.duration_ms}ms` : ""}</td>
      `;
    }
    fragment.appendChild(row);

    // Error row.
    if (job.error) {
      const errorRowId = `job-error-${job.name}`;
      let errorRow = document.getElementById(errorRowId) as HTMLTableRowElement | null;
      if (!errorRow) {
        errorRow = document.createElement("tr");
        errorRow.id = errorRowId;
        errorRow.innerHTML = `<td colspan="3"><pre class="status-failed small">${escapeHtml(job.error)}</pre></td>`;
      }
      fragment.appendChild(errorRow);
    }

    // Stream rows.
    const jobStreams = state.streams[job.name] || [];
    for (const stream of jobStreams) {
      const streamUrl = `output/${state.run.id}/${job.name}/${stream.name}`;
      const existingStreamRow = tbody.querySelector<HTMLElement>(`[data-stream-url="${streamUrl}"]`);

      if (existingStreamRow) {
        // Re-append existing stream row (preserves loaded state).
        const streamTr = existingStreamRow.closest("tr")!;
        // Update the status dot color.
        const dot = streamTr.querySelector<HTMLSpanElement>("summary > span:first-child");
        if (dot) {
          dot.className = statusClass(stream.status);
        }
        fragment.appendChild(streamTr);
      } else {
        // Create new stream viewer row.
        const durationText = stream.duration_ms != null ? `<span class="small" style="margin-left: 0.5rem;">${stream.duration_ms}ms</span>` : "";
        const streamTr = document.createElement("tr");
        streamTr.innerHTML = `
          <td colspan="3" style="padding: 0 0.5rem;">
            <details class="stream-viewer" data-stream-url="${streamUrl}" data-last-line="" data-stream-status="${escapeHtml(stream.status)}">
              <summary class="mono small" style="cursor: pointer; padding: 0.25rem 0;">
                <span class="${statusClass(stream.status)}">&#x25cf;</span>
                ${" "}${escapeHtml(stream.display_name || stream.name)}${durationText}
                <span class="stream-last-line" style="color: #8b949e; margin-left: 0.5rem;"></span>
              </summary>
              <pre class="stream-output" style="max-height: 12em; overflow-y: auto; background: #161b22; border: 1px solid #30363d; border-radius: 3px; padding: 0.5rem; font-size: 0.8125rem;"></pre>
            </details>
          </td>
        `;
        // Initialize the newly created stream viewer.
        const newDetails = streamTr.querySelector<HTMLDetailsElement>(".stream-viewer")!;
        initOneStream(newDetails, running);
        fragment.appendChild(streamTr);
      }
    }
  }

  // Replace tbody contents.
  tbody.innerHTML = "";
  tbody.appendChild(fragment);
}

function escapeHtml(s: string): string {
  const div = document.createElement("div");
  div.textContent = s;
  return div.innerHTML;
}
