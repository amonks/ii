import { Cell, Wall, Marker, ToolName } from "./types";
import { Camera } from "./camera";
import { CELL_SIZE, cellKey, wallKey, nearestWallEdge, pixelToHex } from "./grid";
import * as api from "./api";

export interface AppState {
  mapID: number;
  mapType: "dungeon" | "hex";
  cells: Map<string, Cell>;
  walls: Map<string, Wall>;
  markers: Map<string, Marker>;
  selectedRooms: Set<number>;
  selectedHexes: Set<string>;
  shiftDown: boolean;
  hoveredEdge: { x1: number; y1: number; x2: number; y2: number } | null;
  hoveredEdgeValid: boolean;
  hoveredCell: { x: number; y: number } | null;
  camera: Camera;
  canvas: HTMLCanvasElement;
  requestRender: () => void;
  showProperties: (cells: Cell[]) => void;
  hideProperties: () => void;
  dragPreview: { x1: number; y1: number; x2: number; y2: number } | null;
}

export interface Tool {
  name: ToolName;
  onPointerDown(state: AppState, worldX: number, worldY: number): void;
  onPointerMove(state: AppState, worldX: number, worldY: number): void;
  onPointerUp(state: AppState, worldX: number, worldY: number): void;
}

// --- Select Tool ---

class SelectTool implements Tool {
  name: ToolName = "select";
  private startX = 0;
  private startY = 0;
  private dragging = false;

  onPointerDown(state: AppState, wx: number, wy: number) {
    this.dragging = true;
    if (state.mapType === "dungeon") {
      const gx = Math.floor(wx / CELL_SIZE);
      const gy = Math.floor(wy / CELL_SIZE);
      this.startX = gx;
      this.startY = gy;
    } else {
      const [col, row] = pixelToHex(wx, wy);
      this.startX = col;
      this.startY = row;
    }
  }

  onPointerMove(state: AppState, wx: number, wy: number) {
    if (!this.dragging) return;
    if (state.mapType === "dungeon") {
      const gx = Math.floor(wx / CELL_SIZE);
      const gy = Math.floor(wy / CELL_SIZE);
      state.dragPreview = {
        x1: Math.min(this.startX, gx),
        y1: Math.min(this.startY, gy),
        x2: Math.max(this.startX, gx),
        y2: Math.max(this.startY, gy),
      };
      state.requestRender();
    }
  }

  onPointerUp(state: AppState, wx: number, wy: number) {
    this.dragging = false;
    state.dragPreview = null;

    if (state.mapType === "dungeon") {
      const gx = Math.floor(wx / CELL_SIZE);
      const gy = Math.floor(wy / CELL_SIZE);
      const x1 = Math.min(this.startX, gx);
      const y1 = Math.min(this.startY, gy);
      const x2 = Math.max(this.startX, gx);
      const y2 = Math.max(this.startY, gy);

      // Build roomCells map: room_id → list of cells
      const roomCells = new Map<number, Cell[]>();
      for (const cell of state.cells.values()) {
        if (cell.room_id == null) continue;
        let list = roomCells.get(cell.room_id);
        if (!list) {
          list = [];
          roomCells.set(cell.room_id, list);
        }
        list.push(cell);
      }

      const isSingleCell = x1 === x2 && y1 === y2;

      if (state.shiftDown && isSingleCell) {
        // Shift-click: toggle the clicked room in the existing selection
        const clicked = state.cells.get(cellKey(x1, y1));
        if (clicked?.room_id != null) {
          if (state.selectedRooms.has(clicked.room_id)) {
            state.selectedRooms.delete(clicked.room_id);
          } else {
            state.selectedRooms.add(clicked.room_id);
          }
        }
      } else {
        state.selectedRooms.clear();

        if (isSingleCell) {
          // Click: select the room the clicked cell belongs to
          const clicked = state.cells.get(cellKey(x1, y1));
          if (clicked?.room_id != null) {
            state.selectedRooms.add(clicked.room_id);
          }
        } else {
          // Drag: find rooms fully enclosed by the selection box
          for (const [roomID, cells] of roomCells) {
            const allInside = cells.every(
              (c) => c.x >= x1 && c.x <= x2 && c.y >= y1 && c.y <= y2,
            );
            if (allInside) {
              state.selectedRooms.add(roomID);
            }
          }
        }
      }

      // Collect all cells from selected rooms for the properties panel
      const selectedCells: Cell[] = [];
      for (const roomID of state.selectedRooms) {
        const cells = roomCells.get(roomID);
        if (cells) selectedCells.push(...cells);
      }

      if (selectedCells.length > 0) {
        state.showProperties(selectedCells);
      } else {
        state.hideProperties();
      }
    } else {
      const [col, row] = pixelToHex(wx, wy);
      const key = cellKey(col, row);
      state.selectedRooms.clear();
      state.selectedHexes.clear();
      state.selectedHexes.add(key);
      let cell = state.cells.get(key);
      if (!cell) {
        cell = {
          map_id: state.mapID,
          x: col,
          y: row,
          is_explored: false,
          text: "",
          hue: null,
          room_id: null,
        };
        state.cells.set(key, cell);
      }
      state.showProperties([cell]);
    }

    state.requestRender();
  }
}

// --- Box Tool (Dungeon Mode) ---

class BoxTool implements Tool {
  name: ToolName = "box";
  private startX = 0;
  private startY = 0;
  private dragging = false;

  onPointerDown(state: AppState, wx: number, wy: number) {
    this.dragging = true;
    this.startX = Math.floor(wx / CELL_SIZE);
    this.startY = Math.floor(wy / CELL_SIZE);
    state.dragPreview = { x1: this.startX, y1: this.startY, x2: this.startX, y2: this.startY };
    state.requestRender();
  }

  onPointerMove(state: AppState, wx: number, wy: number) {
    if (!this.dragging) return;
    const gx = Math.floor(wx / CELL_SIZE);
    const gy = Math.floor(wy / CELL_SIZE);
    state.dragPreview = {
      x1: Math.min(this.startX, gx),
      y1: Math.min(this.startY, gy),
      x2: Math.max(this.startX, gx),
      y2: Math.max(this.startY, gy),
    };
    state.requestRender();
  }

  async onPointerUp(state: AppState, wx: number, wy: number) {
    this.dragging = false;
    const gx = Math.floor(wx / CELL_SIZE);
    const gy = Math.floor(wy / CELL_SIZE);
    const x1 = Math.min(this.startX, gx);
    const y1 = Math.min(this.startY, gy);
    const x2 = Math.max(this.startX, gx);
    const y2 = Math.max(this.startY, gy);

    state.dragPreview = null;

    // Check if the drawn box is adjacent to or overlaps the selected room.
    // Adjacent means at least one cell in the box shares an edge (not diagonal)
    // with a cell belonging to the selected room.
    let adjacentToSelected = false;
    if (state.selectedRooms.size > 0) {
      for (let x = x1; x <= x2 && !adjacentToSelected; x++) {
        for (let y = y1; y <= y2 && !adjacentToSelected; y++) {
          // Check if this cell itself belongs to a selected room (overlap)
          const here = state.cells.get(cellKey(x, y));
          if (here?.room_id != null && state.selectedRooms.has(here.room_id)) {
            adjacentToSelected = true;
            break;
          }
          // Check 4 cardinal neighbors for selected room cells
          for (const [dx, dy] of [[0, -1], [0, 1], [-1, 0], [1, 0]]) {
            const neighbor = state.cells.get(cellKey(x + dx, y + dy));
            if (neighbor?.room_id != null && state.selectedRooms.has(neighbor.room_id)) {
              adjacentToSelected = true;
              break;
            }
          }
        }
      }
    }

    if (state.selectedRooms.size > 0 && adjacentToSelected) {
      // Merge mode: assign all cells in drawn box to the first selected room
      const selectedRoomId = state.selectedRooms.values().next().value!;
      const cells: Partial<Cell>[] = [];
      for (let x = x1; x <= x2; x++) {
        for (let y = y1; y <= y2; y++) {
          const existing = state.cells.get(cellKey(x, y));
          cells.push({
            x, y,
            is_explored: true,
            room_id: selectedRoomId,
            text: existing?.text ?? "",
            hue: existing?.hue ?? null,
          });
        }
      }

      const result = await api.upsertCells(state.mapID, cells);
      for (const cell of result) {
        state.cells.set(cellKey(cell.x, cell.y), cell);
      }
    } else {
      // Subtract mode: only assign cells that don't already have a room_id
      let maxRoomID = 0;
      for (const cell of state.cells.values()) {
        if (cell.room_id != null && cell.room_id > maxRoomID) {
          maxRoomID = cell.room_id;
        }
      }

      const cells: Partial<Cell>[] = [];
      for (let x = x1; x <= x2; x++) {
        for (let y = y1; y <= y2; y++) {
          const existing = state.cells.get(cellKey(x, y));
          if (existing?.room_id != null) continue; // skip occupied cells
          cells.push({
            x, y,
            is_explored: true,
            room_id: maxRoomID + 1,
            text: "",
          });
        }
      }

      if (cells.length === 0) {
        state.requestRender();
        return;
      }

      const result = await api.upsertCells(state.mapID, cells);
      for (const cell of result) {
        state.cells.set(cellKey(cell.x, cell.y), cell);
      }
    }

    state.requestRender();
  }
}

// --- Door Tool ---

class DoorTool implements Tool {
  name: ToolName = "door";

  onPointerDown() {}

  onPointerMove(state: AppState, wx: number, wy: number) {
    const edge = nearestWallEdge(wx, wy);
    if (edge) {
      state.hoveredEdge = {
        x1: edge.cellX,
        y1: edge.cellY,
        x2: edge.neighborX,
        y2: edge.neighborY,
      };
      // Valid = this edge is a room boundary (different rooms, or room vs empty)
      const cellA = state.cells.get(cellKey(edge.cellX, edge.cellY));
      const cellB = state.cells.get(cellKey(edge.neighborX, edge.neighborY));
      const roomA = cellA?.room_id ?? null;
      const roomB = cellB?.room_id ?? null;
      // At least one side must have a room, and they must differ
      state.hoveredEdgeValid = (roomA != null || roomB != null) && roomA !== roomB;
    } else {
      state.hoveredEdge = null;
      state.hoveredEdgeValid = false;
    }
    state.requestRender();
  }

  async onPointerUp(state: AppState, wx: number, wy: number) {
    const edge = nearestWallEdge(wx, wy);
    if (!edge) return;

    // Only allow doors on room boundaries
    const cellA = state.cells.get(cellKey(edge.cellX, edge.cellY));
    const cellB = state.cells.get(cellKey(edge.neighborX, edge.neighborY));
    const roomA = cellA?.room_id ?? null;
    const roomB = cellB?.room_id ?? null;
    const isBoundary = (roomA != null || roomB != null) && roomA !== roomB;
    if (!isBoundary) return;

    const wk = wallKey(edge.cellX, edge.cellY, edge.neighborX, edge.neighborY);
    const existing = state.walls.get(wk);

    if (existing?.type === "door") {
      state.walls.delete(wk);
      state.requestRender();
      return;
    }

    const wall: Partial<Wall> = {
      x1: edge.cellX,
      y1: edge.cellY,
      x2: edge.neighborX,
      y2: edge.neighborY,
      type: "door",
    };

    const result = await api.upsertWall(state.mapID, wall);
    state.walls.set(wallKey(result.x1, result.y1, result.x2, result.y2), result);
    state.requestRender();
  }
}

// --- Letter Tool ---

class LetterTool implements Tool {
  name: ToolName = "letter";
  private inputEl: HTMLInputElement | null = null;

  onPointerDown() {}

  onPointerMove(state: AppState, wx: number, wy: number) {
    const gx = Math.floor(wx / CELL_SIZE);
    const gy = Math.floor(wy / CELL_SIZE);
    state.hoveredCell = { x: gx, y: gy };
    state.requestRender();
  }

  async onPointerUp(state: AppState, wx: number, wy: number) {
    const gx = Math.floor(wx / CELL_SIZE);
    const gy = Math.floor(wy / CELL_SIZE);

    const key = cellKey(gx, gy);
    const existing = state.markers.get(key);

    if (existing) {
      await api.deleteMarker(state.mapID, gx, gy);
      state.markers.delete(key);
      state.requestRender();
      return;
    }

    // Show inline input positioned over the cell
    this.removeInput();
    const input = document.createElement("input");
    input.type = "text";
    input.maxLength = 1;
    input.style.cssText = "position:absolute;width:32px;height:32px;font-size:18px;font-weight:bold;text-align:center;background:rgba(30,25,20,0.9);color:#fbbf24;border:2px solid #fbbf24;border-radius:4px;outline:none;padding:0;text-transform:uppercase;z-index:10;";

    // Position the input over the cell using screen coordinates
    const cam = state.camera;
    const [screenX, screenY] = cam.worldToScreen(
      gx * CELL_SIZE + CELL_SIZE / 2,
      gy * CELL_SIZE + CELL_SIZE / 2,
    );
    const canvasRect = state.canvas.getBoundingClientRect();
    input.style.left = `${canvasRect.left + screenX - 16}px`;
    input.style.top = `${canvasRect.top + screenY - 16}px`;

    const commit = async () => {
      const letter = input.value.trim();
      this.removeInput();
      if (!letter) return;

      const marker: Partial<Marker> = {
        x: gx,
        y: gy,
        letter: letter[0].toUpperCase(),
      };
      const result = await api.upsertMarker(state.mapID, marker);
      state.markers.set(cellKey(result.x, result.y), result);
      state.requestRender();
    };

    input.addEventListener("keydown", (e) => {
      if (e.key === "Enter") commit();
      if (e.key === "Escape") this.removeInput();
    });
    input.addEventListener("blur", () => commit());

    document.body.appendChild(input);
    input.focus();
    this.inputEl = input;
  }

  private removeInput() {
    if (this.inputEl) {
      this.inputEl.remove();
      this.inputEl = null;
    }
  }
}

// --- Paint Tool (Hex Mode) ---

class PaintTool implements Tool {
  name: ToolName = "paint";
  private painting = false;

  onPointerDown(state: AppState, wx: number, wy: number) {
    this.painting = true;
    this.paintHex(state, wx, wy);
  }

  onPointerMove(state: AppState, wx: number, wy: number) {
    if (!this.painting) return;
    this.paintHex(state, wx, wy);
  }

  onPointerUp() {
    this.painting = false;
  }

  private async paintHex(state: AppState, wx: number, wy: number) {
    const [col, row] = pixelToHex(wx, wy);
    const key = cellKey(col, row);
    const existing = state.cells.get(key);

    const cell: Partial<Cell> = {
      x: col,
      y: row,
      is_explored: existing ? !existing.is_explored : true,
      text: existing?.text ?? "",
      hue: existing?.hue ?? null,
    };

    // Optimistic update
    const optimistic: Cell = {
      map_id: state.mapID,
      x: col,
      y: row,
      is_explored: cell.is_explored!,
      text: cell.text!,
      hue: cell.hue ?? null,
      room_id: null,
    };
    state.cells.set(key, optimistic);
    state.requestRender();

    const result = await api.upsertCells(state.mapID, [cell]);
    if (result.length > 0) {
      state.cells.set(cellKey(result[0].x, result[0].y), result[0]);
      state.requestRender();
    }
  }
}

// --- Tool Registry ---

export const TOOLS: Record<ToolName, Tool> = {
  select: new SelectTool(),
  box: new BoxTool(),
  door: new DoorTool(),
  letter: new LetterTool(),
  paint: new PaintTool(),
};
