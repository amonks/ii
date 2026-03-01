import { Cell } from "./types";
import { cellKey } from "./grid";
import * as api from "./api";
import { AppState } from "./tools";

const HUE_SWATCHES = [
  { label: "Brown", hue: 30 },
  { label: "Red", hue: 0 },
  { label: "Orange", hue: 25 },
  { label: "Yellow", hue: 50 },
  { label: "Green", hue: 120 },
  { label: "Teal", hue: 180 },
  { label: "Blue", hue: 220 },
  { label: "Purple", hue: 270 },
  { label: "Pink", hue: 330 },
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

  // Explored toggle
  const exploredDiv = document.createElement("div");
  exploredDiv.style.cssText = "display:flex;align-items:center;gap:8px;";
  const exploredCheck = document.createElement("input");
  exploredCheck.type = "checkbox";
  exploredCheck.checked = cells.every((c) => c.is_explored);
  exploredCheck.style.cssText = "width:18px;height:18px;accent-color:#d97706;cursor:pointer;";
  exploredCheck.addEventListener("change", async () => {
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

  // "None" swatch
  const noneSwatch = document.createElement("button");
  noneSwatch.style.cssText = "width:28px;height:28px;border-radius:4px;border:2px solid #57534e;background:#44403c;color:#a8a29e;font-size:16px;cursor:pointer;display:flex;align-items:center;justify-content:center;";
  noneSwatch.title = "None";
  noneSwatch.innerHTML = "&times;";
  noneSwatch.addEventListener("click", async () => {
    const updates: Partial<Cell>[] = cells.map((c) => ({
      x: c.x, y: c.y, is_explored: c.is_explored, text: c.text,
      hue: null, room_id: c.room_id,
    }));
    const result = await api.upsertCells(state.mapID, updates);
    for (const cell of result) {
      state.cells.set(cellKey(cell.x, cell.y), cell);
    }
    state.requestRender();
  });
  swatchRow.appendChild(noneSwatch);

  const currentHue = cells[0]?.hue;
  for (const sw of HUE_SWATCHES) {
    const btn = document.createElement("button");
    const isActive = currentHue === sw.hue;
    btn.style.cssText = `width:28px;height:28px;border-radius:4px;border:2px solid ${isActive ? "#fbbf24" : "#57534e"};background:hsl(${sw.hue},40%,40%);cursor:pointer;`;
    if (isActive) {
      btn.style.boxShadow = "0 0 0 2px #fbbf24";
    }
    btn.title = sw.label;
    btn.addEventListener("click", async () => {
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
  const textInput = document.createElement("input");
  textInput.type = "text";
  textInput.value = cells.length === 1 ? cells[0].text : "";
  textInput.placeholder = "Add notes...";
  textInput.style.cssText = "width:100%;background:#44403c;border:1px solid #57534e;border-radius:4px;padding:4px 8px;font-size:14px;color:#e7e5e4;outline:none;box-sizing:border-box;";
  let textTimeout: ReturnType<typeof setTimeout>;
  textInput.addEventListener("input", () => {
    clearTimeout(textTimeout);
    textTimeout = setTimeout(async () => {
      const updates: Partial<Cell>[] = cells.map((c) => ({
        x: c.x, y: c.y, is_explored: c.is_explored, text: textInput.value,
        hue: c.hue, room_id: c.room_id,
      }));
      const result = await api.upsertCells(state.mapID, updates);
      for (const cell of result) {
        state.cells.set(cellKey(cell.x, cell.y), cell);
      }
      state.requestRender();
    }, 300);
  });
  textDiv.appendChild(textLabel);
  textDiv.appendChild(textInput);
  content.appendChild(textDiv);
}

export function hideProperties() {
  const panel = document.getElementById("properties")!;
  panel.classList.add("hidden");
}
