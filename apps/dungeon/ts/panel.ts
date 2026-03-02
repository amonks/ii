import { Cell, Wall } from "./types";
import { cellKey, wallKey } from "./grid";
import * as api from "./api";
import { AppState } from "./tools";
import { UndoEntry } from "./undo";

const HUE_SWATCHES = [
  { label: "Red", hue: 0 },
  { label: "Orange", hue: 40 },
  { label: "Yellow", hue: 80 },
  { label: "Green", hue: 120 },
  { label: "Teal", hue: 160 },
  { label: "Cyan", hue: 200 },
  { label: "Blue", hue: 240 },
  { label: "Purple", hue: 280 },
  { label: "Pink", hue: 320 },
];

export function showProperties(state: AppState, cells: Cell[]) {
  const panel = document.getElementById("properties")!;
  const title = document.getElementById("prop-title")!;
  const content = document.getElementById("prop-content")!;

  panel.classList.remove("hidden");

  // Determine room-based title
  const roomIds = new Set<number>();
  for (const c of cells) {
    if (c.room_id != null) roomIds.add(c.room_id);
  }
  if (roomIds.size === 1) {
    const roomId = roomIds.values().next().value!;
    title.textContent = `Room ${roomId} (${cells.length} cells)`;
  } else if (roomIds.size > 1) {
    title.textContent = `${roomIds.size} rooms selected`;
  } else if (cells.length === 1) {
    title.textContent = `Cell (${cells[0].x}, ${cells[0].y})`;
  } else {
    title.textContent = `${cells.length} cells selected`;
  }

  content.innerHTML = "";

  // Merge button (when multiple rooms are selected)
  if (roomIds.size > 1) {
    const mergeBtn = document.createElement("button");
    mergeBtn.textContent = "Merge Rooms";
    mergeBtn.style.cssText = "width:100%;padding:6px 12px;background:#d97706;color:#1c1917;border:none;border-radius:4px;font-size:14px;font-weight:600;cursor:pointer;";
    mergeBtn.addEventListener("click", async () => {
      // Snapshot for undo
      const undoCells = new Map<string, Cell | null>();
      for (const c of cells) {
        undoCells.set(cellKey(c.x, c.y), { ...c });
      }
      state.pushUndo({ cells: undoCells, walls: new Map(), markers: new Map() });

      const targetRoomId = Math.min(...roomIds);
      const updates: Partial<Cell>[] = cells
        .filter((c) => c.room_id !== targetRoomId)
        .map((c) => ({
          x: c.x, y: c.y, is_explored: c.is_explored, text: c.text,
          hue: c.hue, room_id: targetRoomId,
        }));
      if (updates.length === 0) return;
      const result = await api.upsertCells(state.mapID, updates);
      for (const cell of result) {
        state.cells.set(cellKey(cell.x, cell.y), cell);
      }
      // Update selection to just the merged room
      state.selectedRooms.clear();
      state.selectedRooms.add(targetRoomId);
      // Re-render panel with merged cells
      const mergedCells: Cell[] = [];
      for (const cell of state.cells.values()) {
        if (cell.room_id === targetRoomId) mergedCells.push(cell);
      }
      showProperties(state, mergedCells);
      state.requestRender();
    });
    content.appendChild(mergeBtn);
  }

  // Explored toggle
  const exploredDiv = document.createElement("div");
  exploredDiv.style.cssText = "display:flex;align-items:center;gap:8px;";
  const exploredCheck = document.createElement("input");
  exploredCheck.type = "checkbox";
  exploredCheck.checked = cells.every((c) => c.is_explored);
  exploredCheck.style.cssText = "width:18px;height:18px;accent-color:#d97706;cursor:pointer;";
  exploredCheck.addEventListener("change", async () => {
    // Snapshot for undo
    const undoCells = new Map<string, Cell | null>();
    for (const c of cells) {
      undoCells.set(cellKey(c.x, c.y), { ...c });
    }
    state.pushUndo({ cells: undoCells, walls: new Map(), markers: new Map() });

    const updates: Partial<Cell>[] = cells.map((c) => ({
      x: c.x,
      y: c.y,
      is_explored: exploredCheck.checked,
      text: c.text,
      hue: c.hue,
      room_id: c.room_id,
    }));
    const result = await api.upsertCells(state.mapID, updates);
    for (const cell of result) {
      state.cells.set(cellKey(cell.x, cell.y), cell);
    }
    state.requestRender();
  });
  const exploredLabel = document.createElement("label");
  exploredLabel.textContent = "Explored";
  exploredLabel.style.cssText = "font-size:14px;color:#a8a29e;cursor:pointer;";
  exploredLabel.addEventListener("click", () => exploredCheck.click());
  exploredDiv.appendChild(exploredCheck);
  exploredDiv.appendChild(exploredLabel);
  content.appendChild(exploredDiv);

  // Hue swatches
  const hueDiv = document.createElement("div");
  const hueLabel = document.createElement("div");
  hueLabel.textContent = "Color";
  hueLabel.style.cssText = "font-size:14px;color:#a8a29e;margin-bottom:4px;";
  hueDiv.appendChild(hueLabel);

  const swatchRow = document.createElement("div");
  swatchRow.style.cssText = "display:flex;gap:4px;flex-wrap:wrap;";

  const currentHue = cells[0]?.hue ?? 40;
  for (const sw of HUE_SWATCHES) {
    const btn = document.createElement("button");
    const isActive = currentHue === sw.hue;
    btn.style.cssText = `width:28px;height:28px;border-radius:4px;border:2px solid ${isActive ? "#fbbf24" : "#57534e"};background:hsl(${sw.hue},40%,40%);cursor:pointer;`;
    if (isActive) {
      btn.style.boxShadow = "0 0 0 2px #fbbf24";
    }
    btn.title = sw.label;
    btn.addEventListener("click", async () => {
      // Snapshot for undo
      const undoCells = new Map<string, Cell | null>();
      for (const c of cells) {
        undoCells.set(cellKey(c.x, c.y), { ...c });
      }
      state.pushUndo({ cells: undoCells, walls: new Map(), markers: new Map() });

      const updates: Partial<Cell>[] = cells.map((c) => ({
        x: c.x, y: c.y, is_explored: c.is_explored, text: c.text,
        hue: sw.hue, room_id: c.room_id,
      }));
      const result = await api.upsertCells(state.mapID, updates);
      for (const cell of result) {
        state.cells.set(cellKey(cell.x, cell.y), cell);
      }
      // Re-render the panel to update active swatch
      const updatedCells: Cell[] = [];
      for (const cell of state.cells.values()) {
        if (cell.room_id != null && state.selectedRooms.has(cell.room_id)) {
          updatedCells.push(cell);
        }
      }
      if (updatedCells.length > 0) {
        showProperties(state, updatedCells);
      }
      state.requestRender();
    });
    swatchRow.appendChild(btn);
  }

  hueDiv.appendChild(swatchRow);
  content.appendChild(hueDiv);

  // Text field
  const textDiv = document.createElement("div");
  const textLabel = document.createElement("label");
  textLabel.textContent = "Notes";
  textLabel.style.cssText = "font-size:14px;color:#a8a29e;display:block;margin-bottom:4px;";
  const textInput = document.createElement("textarea");
  textInput.rows = 3;
  // Show text if all cells share the same value
  const allSameText = cells.every((c) => c.text === cells[0].text);
  textInput.value = allSameText ? cells[0].text : "";
  textInput.placeholder = "Add notes...";
  textInput.style.cssText = "width:100%;background:#44403c;border:1px solid #57534e;border-radius:4px;padding:4px 8px;font-size:14px;color:#e7e5e4;outline:none;box-sizing:border-box;resize:vertical;font-family:inherit;";
  let textTimeout: ReturnType<typeof setTimeout>;
  let textUndoPushed = false;
  textInput.addEventListener("input", () => {
    clearTimeout(textTimeout);
    // Snapshot for undo once per editing session
    if (!textUndoPushed) {
      const undoCells = new Map<string, Cell | null>();
      for (const c of cells) {
        undoCells.set(cellKey(c.x, c.y), { ...c });
      }
      state.pushUndo({ cells: undoCells, walls: new Map(), markers: new Map() });
      textUndoPushed = true;
    }
    textTimeout = setTimeout(flushText, 300);
  });
  async function flushText() {
    clearTimeout(textTimeout);
    const updates: Partial<Cell>[] = cells.map((c) => ({
      x: c.x, y: c.y, is_explored: c.is_explored, text: textInput.value,
      hue: c.hue, room_id: c.room_id,
    }));
    const result = await api.upsertCells(state.mapID, updates);
    for (const cell of result) {
      state.cells.set(cellKey(cell.x, cell.y), cell);
    }
    state.requestRender();
  }
  textInput.addEventListener("blur", () => flushText());
  textDiv.appendChild(textLabel);
  textDiv.appendChild(textInput);
  content.appendChild(textDiv);

  // Delete room button
  if (roomIds.size > 0) {
    const deleteBtn = document.createElement("button");
    deleteBtn.textContent = roomIds.size === 1 ? "Delete Room" : "Delete Rooms";
    deleteBtn.style.cssText = "width:100%;padding:6px 12px;background:#dc2626;color:#fef2f2;border:none;border-radius:4px;font-size:14px;font-weight:600;cursor:pointer;margin-top:4px;";
    deleteBtn.addEventListener("click", async () => {
      // Snapshot for undo: cells + their walls
      const undoCells = new Map<string, Cell | null>();
      const undoWalls = new Map<string, Wall | null>();
      const affectedKeys = new Set<string>();
      for (const c of cells) {
        const key = cellKey(c.x, c.y);
        undoCells.set(key, { ...c });
        affectedKeys.add(key);
      }
      for (const [wk, wall] of state.walls) {
        const keyA = cellKey(wall.x1, wall.y1);
        const keyB = cellKey(wall.x2, wall.y2);
        if (affectedKeys.has(keyA) || affectedKeys.has(keyB)) {
          undoWalls.set(wk, { ...wall });
        }
      }
      state.pushUndo({ cells: undoCells, walls: undoWalls, markers: new Map() });

      const coords = cells.map((c) => ({ x: c.x, y: c.y }));
      await api.deleteCells(state.mapID, coords);
      for (const c of cells) {
        state.cells.delete(cellKey(c.x, c.y));
      }
      // Clean up orphaned walls locally (server does the same)
      for (const [wk, wall] of state.walls) {
        const cellA = state.cells.get(cellKey(wall.x1, wall.y1));
        const cellB = state.cells.get(cellKey(wall.x2, wall.y2));
        if ((cellA?.room_id == null) && (cellB?.room_id == null)) {
          state.walls.delete(wk);
        }
      }
      state.selectedRooms.clear();
      hideProperties();
      state.requestRender();
    });
    content.appendChild(deleteBtn);
  }
}

export function hideProperties() {
  const panel = document.getElementById("properties")!;
  panel.classList.add("hidden");
}
