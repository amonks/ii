// @vitest-environment jsdom
import { describe, it, expect, beforeEach } from "vitest";
import { applyRunState, RunState } from "./sse";

function makeState(overrides: Partial<RunState> = {}): RunState {
  return {
    run: {
      id: 1,
      status: "running",
      head_sha: "abc123",
      base_sha: "def456",
      trigger: "webhook",
      started_at: "2024-01-01T00:00:00Z",
      ...overrides.run,
    },
    jobs: overrides.jobs || [],
    streams: overrides.streams || {},
  };
}

describe("applyRunState", () => {
  beforeEach(() => {
    document.body.innerHTML = `
      <div id="run-page" data-run-id="1" data-run-status="running">
        <span id="run-status" class="status-running">running</span>
        <span id="run-finished" style="display: none;"></span>
        <span id="run-error" style="display: none;"></span>
        <p id="no-jobs">No jobs yet.</p>
        <table id="jobs-table">
          <tbody id="jobs-tbody"></tbody>
        </table>
      </div>
    `;
  });

  it("updates run status", () => {
    applyRunState(makeState({ run: { id: 1, status: "success", head_sha: "abc", base_sha: "def", trigger: "webhook", started_at: "t", finished_at: "t2" } }));

    const el = document.getElementById("run-status")!;
    expect(el.textContent).toBe("success");
    expect(el.className).toBe("status-success");
  });

  it("shows finished time", () => {
    applyRunState(makeState({ run: { id: 1, status: "success", head_sha: "a", base_sha: "b", trigger: "w", started_at: "t", finished_at: "2024-01-01T01:00:00Z" } }));

    const el = document.getElementById("run-finished")!;
    expect(el.textContent).toBe("2024-01-01T01:00:00Z");
    expect(el.style.display).toBe("");
  });

  it("shows error", () => {
    applyRunState(makeState({ run: { id: 1, status: "failed", head_sha: "a", base_sha: "b", trigger: "w", started_at: "t", error: "build failed" } }));

    const el = document.getElementById("run-error")!;
    expect(el.textContent).toBe("build failed");
  });

  it("adds job rows", () => {
    applyRunState(makeState({
      jobs: [
        { name: "go-test", kind: "test", status: "in_progress" },
        { name: "deploy", kind: "deploy", status: "success", duration_ms: 1500 },
      ],
    }));

    const rows = document.querySelectorAll("#jobs-tbody tr[data-job-name]");
    expect(rows.length).toBe(2);

    const firstRow = rows[0] as HTMLTableRowElement;
    expect(firstRow.dataset.jobName).toBe("go-test");
    const cells = firstRow.querySelectorAll("td");
    // No Kind column: Name(0), Status(1), Duration(2)
    expect(cells[1].textContent).toBe("in_progress");
    expect(cells[1].className).toBe("status-in_progress");

    const secondRow = rows[1] as HTMLTableRowElement;
    const secondCells = secondRow.querySelectorAll("td");
    expect(secondCells[2].textContent).toBe("1500ms");
  });

  it("updates existing job rows in place", () => {
    // First update: job is in_progress.
    applyRunState(makeState({
      jobs: [{ name: "go-test", kind: "test", status: "in_progress" }],
    }));

    // Second update: job finishes.
    applyRunState(makeState({
      jobs: [{ name: "go-test", kind: "test", status: "success", duration_ms: 2000 }],
    }));

    const rows = document.querySelectorAll("#jobs-tbody tr[data-job-name]");
    expect(rows.length).toBe(1);
    const cells = (rows[0] as HTMLTableRowElement).querySelectorAll("td");
    // No Kind column: Name(0), Status(1), Duration(2)
    expect(cells[1].textContent).toBe("success");
    expect(cells[2].textContent).toBe("2000ms");
  });

  it("adds stream viewer rows", () => {
    applyRunState(makeState({
      jobs: [{ name: "go-test", kind: "test", status: "in_progress" }],
      streams: { "go-test": [
        { name: "stdout", status: "in_progress" },
        { name: "stderr", status: "in_progress" },
      ] },
    }));

    const viewers = document.querySelectorAll(".stream-viewer");
    expect(viewers.length).toBe(2);

    const first = viewers[0] as HTMLDetailsElement;
    expect(first.dataset.streamUrl).toBe("output/1/go-test/stdout");
  });

  it("hides no-jobs message when jobs appear", () => {
    applyRunState(makeState({
      jobs: [{ name: "go-test", kind: "test", status: "in_progress" }],
    }));

    const noJobs = document.getElementById("no-jobs")!;
    expect(noJobs.style.display).toBe("none");
  });

  it("shows job error row", () => {
    applyRunState(makeState({
      jobs: [{ name: "go-test", kind: "test", status: "failed", error: "tests failed" }],
    }));

    const errorRow = document.getElementById("job-error-go-test");
    expect(errorRow).not.toBeNull();
    expect(errorRow!.textContent).toContain("tests failed");
  });
});
