import { describe, it, expect, vi, beforeEach } from "vitest";

vi.mock("./api", () => ({
  upsertCells: vi.fn((_mapID: number, cells: any[]) =>
    Promise.resolve(
      cells.map((c: any) => ({
        id: 1,
        map_id: _mapID,
        x: c.x,
        y: c.y,
        is_explored: c.is_explored ?? false,
        text: c.text ?? "",
        hue: c.hue ?? null,
        room_id: c.room_id ?? null,
      })),
    ),
  ),
  upsertWall: vi.fn(),
  upsertMarker: vi.fn(),
  deleteMarker: vi.fn(),
}));

import { TOOLS, AppState } from "./tools";
import { cellKey, CELL_SIZE } from "./grid";
import { Cell } from "./types";

function makeState(cells: Cell[] = []): AppState {
  const cellMap = new Map<string, Cell>();
  for (const c of cells) {
    cellMap.set(cellKey(c.x, c.y), c);
  }
  return {
    mapID: 1,
    mapType: "dungeon",
    cells: cellMap,
    walls: new Map(),
    markers: new Map(),
    selectedRooms: new Set(),
    hoveredEdge: null,
    camera: { logicalWidth: 800, logicalHeight: 600 } as any,
    canvas: {} as any,
    requestRender: vi.fn(),
    showProperties: vi.fn(),
    hideProperties: vi.fn(),
    dragPreview: null,
  };
}

function makeCell(x: number, y: number, roomId: number | null): Cell {
  return { map_id: 1, x, y, is_explored: true, text: "", hue: null, room_id: roomId };
}

// Convert grid coordinates to world coordinates (center of cell)
function toWorld(gx: number, gy: number): [number, number] {
  return [gx * CELL_SIZE + CELL_SIZE / 2, gy * CELL_SIZE + CELL_SIZE / 2];
}

describe("SelectTool", () => {
  const select = TOOLS.select;

  it("selects rooms fully enclosed by the drag box", () => {
    // Room 1 at (0,0)-(1,1), Room 2 at (5,5)-(6,6)
    const state = makeState([
      makeCell(0, 0, 1), makeCell(1, 0, 1), makeCell(0, 1, 1), makeCell(1, 1, 1),
      makeCell(5, 5, 2), makeCell(6, 5, 2), makeCell(5, 6, 2), makeCell(6, 6, 2),
    ]);

    // Drag from (-1,-1) to (2,2) — fully encloses room 1 but not room 2
    select.onPointerDown(state, ...toWorld(-1, -1));
    select.onPointerUp(state, ...toWorld(2, 2));

    expect(state.selectedRooms.has(1)).toBe(true);
    expect(state.selectedRooms.has(2)).toBe(false);
    expect(state.showProperties).toHaveBeenCalled();
  });

  it("does not select rooms only partially inside the box", () => {
    // Room 1 spans (0,0)-(2,2)
    const state = makeState([
      makeCell(0, 0, 1), makeCell(1, 0, 1), makeCell(2, 0, 1),
      makeCell(0, 1, 1), makeCell(1, 1, 1), makeCell(2, 1, 1),
      makeCell(0, 2, 1), makeCell(1, 2, 1), makeCell(2, 2, 1),
    ]);

    // Drag from (0,0) to (1,1) — only covers part of room 1
    select.onPointerDown(state, ...toWorld(0, 0));
    select.onPointerUp(state, ...toWorld(1, 1));

    expect(state.selectedRooms.size).toBe(0);
    expect(state.hideProperties).toHaveBeenCalled();
  });

  it("selects multiple rooms when all are fully enclosed", () => {
    const state = makeState([
      makeCell(0, 0, 1), makeCell(1, 0, 1),
      makeCell(3, 0, 2), makeCell(4, 0, 2),
    ]);

    // Drag from (-1,-1) to (5,1) — covers both rooms
    select.onPointerDown(state, ...toWorld(-1, -1));
    select.onPointerUp(state, ...toWorld(5, 1));

    expect(state.selectedRooms.has(1)).toBe(true);
    expect(state.selectedRooms.has(2)).toBe(true);
  });
});

describe("BoxTool", () => {
  const box = TOOLS.box;

  it("creates a new room when no room is selected (subtract mode)", async () => {
    const state = makeState();

    box.onPointerDown(state, ...toWorld(0, 0));
    await box.onPointerUp(state, ...toWorld(1, 1));

    // Should have created cells for a 2x2 room
    expect(state.cells.size).toBe(4);
    for (const cell of state.cells.values()) {
      expect(cell.room_id).toBe(1);
      expect(cell.is_explored).toBe(true);
    }
  });

  it("skips occupied cells in subtract mode", async () => {
    // Pre-existing room 1 at (1,1)
    const state = makeState([makeCell(1, 1, 1)]);

    box.onPointerDown(state, ...toWorld(0, 0));
    await box.onPointerUp(state, ...toWorld(2, 2));

    // (1,1) should still belong to room 1 (the API mock returns room_id from the input)
    // The new cells should be room 2
    const cell11 = state.cells.get(cellKey(1, 1))!;
    expect(cell11.room_id).toBe(1);

    // All other cells in the 3x3 box should be room 2
    const newRoomCells = Array.from(state.cells.values()).filter(c => c.room_id === 2);
    expect(newRoomCells.length).toBe(8); // 9 cells - 1 occupied = 8
  });

  it("creates ring-shaped rooms via concentric boxes", async () => {
    const state = makeState();

    // Draw outer 5x5 box
    box.onPointerDown(state, ...toWorld(0, 0));
    await box.onPointerUp(state, ...toWorld(4, 4));

    expect(state.cells.size).toBe(25);

    // Draw inner 3x3 box (no room selected, subtract mode)
    // Cells (1,1)-(3,3) are already occupied by room 1, so they're skipped
    box.onPointerDown(state, ...toWorld(1, 1));
    await box.onPointerUp(state, ...toWorld(3, 3));

    // Inner cells should remain room 1 (skipped)
    const innerCell = state.cells.get(cellKey(2, 2))!;
    expect(innerCell.room_id).toBe(1);

    // All cells should be room 1 (nothing was unoccupied in inner box)
    const room1Cells = Array.from(state.cells.values()).filter(c => c.room_id === 1);
    expect(room1Cells.length).toBe(25);
  });

  it("merges cells into selected room (merge mode)", async () => {
    // Room 1 at (0,0)-(1,1)
    const state = makeState([
      makeCell(0, 0, 1), makeCell(1, 0, 1),
      makeCell(0, 1, 1), makeCell(1, 1, 1),
    ]);

    // Select room 1
    state.selectedRooms.add(1);

    // Draw box at (2,0)-(3,1) — adjacent to room 1
    box.onPointerDown(state, ...toWorld(2, 0));
    await box.onPointerUp(state, ...toWorld(3, 1));

    // New cells should be room 1
    const cell20 = state.cells.get(cellKey(2, 0))!;
    expect(cell20.room_id).toBe(1);
    const cell31 = state.cells.get(cellKey(3, 1))!;
    expect(cell31.room_id).toBe(1);

    // Total room 1 cells should be 8
    const room1Cells = Array.from(state.cells.values()).filter(c => c.room_id === 1);
    expect(room1Cells.length).toBe(8);
  });

  it("reassigns cells from other rooms in merge mode", async () => {
    // Room 1 at (0,0)-(1,0), Room 2 at (2,0)-(3,0)
    const state = makeState([
      makeCell(0, 0, 1), makeCell(1, 0, 1),
      makeCell(2, 0, 2), makeCell(3, 0, 2),
    ]);

    // Select room 1
    state.selectedRooms.add(1);

    // Draw box over room 2's cells
    box.onPointerDown(state, ...toWorld(2, 0));
    await box.onPointerUp(state, ...toWorld(3, 0));

    // Room 2 cells should now be room 1
    const cell20 = state.cells.get(cellKey(2, 0))!;
    expect(cell20.room_id).toBe(1);
    const cell30 = state.cells.get(cellKey(3, 0))!;
    expect(cell30.room_id).toBe(1);
  });
});
