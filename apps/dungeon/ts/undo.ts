import { Cell, Wall, Marker } from "./types";
import { cellKey, wallKey } from "./grid";
import * as api from "./api";
import { AppState } from "./tools";

export interface UndoEntry {
  cells: Map<string, Cell | null>;   // key → previous state (null = didn't exist)
  walls: Map<string, Wall | null>;
  markers: Map<string, Marker | null>;
}

export class UndoStack {
  private undoStack: UndoEntry[] = [];
  private redoStack: UndoEntry[] = [];

  push(entry: UndoEntry) {
    this.undoStack.push(entry);
    this.redoStack = [];
  }

  get canUndo(): boolean {
    return this.undoStack.length > 0;
  }

  get canRedo(): boolean {
    return this.redoStack.length > 0;
  }

  async undo(state: AppState) {
    const entry = this.undoStack.pop();
    if (!entry) return;

    // Save current state of the same keys as a redo entry
    const redoEntry = this.snapshot(state, entry);
    this.redoStack.push(redoEntry);

    await this.restore(state, entry);
  }

  async redo(state: AppState) {
    const entry = this.redoStack.pop();
    if (!entry) return;

    // Save current state of the same keys as an undo entry
    const undoEntry = this.snapshot(state, entry);
    this.undoStack.push(undoEntry);

    await this.restore(state, entry);
  }

  private snapshot(state: AppState, entry: UndoEntry): UndoEntry {
    const cells = new Map<string, Cell | null>();
    const walls = new Map<string, Wall | null>();
    const markers = new Map<string, Marker | null>();

    for (const key of entry.cells.keys()) {
      cells.set(key, state.cells.get(key) ?? null);
    }
    for (const key of entry.walls.keys()) {
      walls.set(key, state.walls.get(key) ?? null);
    }
    for (const key of entry.markers.keys()) {
      markers.set(key, state.markers.get(key) ?? null);
    }

    return { cells, walls, markers };
  }

  private async restore(state: AppState, entry: UndoEntry) {
    // Restore cells
    const cellsToUpsert: Partial<Cell>[] = [];
    const cellsToDelete: { x: number; y: number }[] = [];

    for (const [key, cell] of entry.cells) {
      if (cell === null) {
        const [xs, ys] = key.split(",");
        cellsToDelete.push({ x: parseInt(xs, 10), y: parseInt(ys, 10) });
        state.cells.delete(key);
      } else {
        cellsToUpsert.push({
          x: cell.x, y: cell.y,
          is_explored: cell.is_explored,
          text: cell.text,
          hue: cell.hue,
          room_id: cell.room_id,
        });
        state.cells.set(key, cell);
      }
    }

    if (cellsToUpsert.length > 0) {
      const result = await api.upsertCells(state.mapID, cellsToUpsert);
      for (const cell of result) {
        state.cells.set(cellKey(cell.x, cell.y), cell);
      }
    }
    if (cellsToDelete.length > 0) {
      await api.deleteCells(state.mapID, cellsToDelete);
    }

    // Restore walls
    for (const [key, wall] of entry.walls) {
      if (wall === null) {
        state.walls.delete(key);
      } else {
        const result = await api.upsertWall(state.mapID, {
          x1: wall.x1, y1: wall.y1,
          x2: wall.x2, y2: wall.y2,
          type: wall.type,
        });
        state.walls.set(wallKey(result.x1, result.y1, result.x2, result.y2), result);
      }
    }

    // Restore markers
    for (const [key, marker] of entry.markers) {
      if (marker === null) {
        const [xs, ys] = key.split(",");
        await api.deleteMarker(state.mapID, parseInt(xs, 10), parseInt(ys, 10));
        state.markers.delete(key);
      } else {
        const result = await api.upsertMarker(state.mapID, {
          x: marker.x, y: marker.y,
          letter: marker.letter,
        });
        state.markers.set(cellKey(result.x, result.y), result);
      }
    }

    // Clean up orphaned walls after cell deletions
    if (cellsToDelete.length > 0) {
      for (const [wk, wall] of state.walls) {
        const cellA = state.cells.get(cellKey(wall.x1, wall.y1));
        const cellB = state.cells.get(cellKey(wall.x2, wall.y2));
        if ((cellA?.room_id == null) && (cellB?.room_id == null)) {
          state.walls.delete(wk);
        }
      }
    }

    state.requestRender();
  }
}

// Helper to create an undo entry that snapshots the current state of cells in a rectangle
export function snapshotCells(state: AppState, x1: number, y1: number, x2: number, y2: number): UndoEntry {
  const cells = new Map<string, Cell | null>();
  for (let x = x1; x <= x2; x++) {
    for (let y = y1; y <= y2; y++) {
      const key = cellKey(x, y);
      cells.set(key, state.cells.get(key) ?? null);
    }
  }
  return { cells, walls: new Map(), markers: new Map() };
}

// Snapshot specific cell keys and their associated walls
export function snapshotCellsAndWalls(state: AppState, cellKeys: string[]): UndoEntry {
  const cells = new Map<string, Cell | null>();
  const walls = new Map<string, Wall | null>();

  for (const key of cellKeys) {
    cells.set(key, state.cells.get(key) ?? null);
  }

  // Snapshot walls that touch any of the affected cells
  const affectedCoords = new Set(cellKeys);
  for (const [wk, wall] of state.walls) {
    const keyA = cellKey(wall.x1, wall.y1);
    const keyB = cellKey(wall.x2, wall.y2);
    if (affectedCoords.has(keyA) || affectedCoords.has(keyB)) {
      walls.set(wk, wall);
    }
  }

  return { cells, walls, markers: new Map() };
}
