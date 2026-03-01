import { Cell, Wall, Marker } from "./types";
import { Camera } from "./camera";

export const CELL_SIZE = 40;

// --- Square Grid (Dungeon Mode) ---

export function drawDungeonGrid(
  ctx: CanvasRenderingContext2D,
  camera: Camera,
  cells: Map<string, Cell>,
  walls: Map<string, Wall>,
  markers: Map<string, Marker>,
  selectedRooms: Set<number> | null,
  dragPreview: { x1: number; y1: number; x2: number; y2: number } | null,
  hoveredEdge: { x1: number; y1: number; x2: number; y2: number } | null,
  hoveredEdgeValid: boolean,
  hoveredCell: { x: number; y: number } | null,
) {
  ctx.save();
  camera.apply(ctx);

  // Determine visible range using CSS pixel screen bounds
  const [tlx, tly] = camera.screenToWorld(0, 0);
  const [brx, bry] = camera.screenToWorld(camera.logicalWidth, camera.logicalHeight);
  const minGX = Math.floor(tlx / CELL_SIZE) - 1;
  const minGY = Math.floor(tly / CELL_SIZE) - 1;
  const maxGX = Math.ceil(brx / CELL_SIZE) + 1;
  const maxGY = Math.ceil(bry / CELL_SIZE) + 1;

  // Draw background grid dots
  ctx.fillStyle = "#57534e";
  for (let gx = minGX; gx <= maxGX; gx++) {
    for (let gy = minGY; gy <= maxGY; gy++) {
      ctx.fillRect(gx * CELL_SIZE - 1, gy * CELL_SIZE - 1, 2, 2);
    }
  }

  // Draw brighter floor dots inside rooms (on top of background)
  const roomDotPositions = new Set<string>();
  for (const cell of cells.values()) {
    if (cell.room_id == null) continue;
    for (const [dx, dy] of [[0, 0], [1, 0], [0, 1], [1, 1]]) {
      const gx = cell.x + dx;
      const gy = cell.y + dy;
      roomDotPositions.add(`${gx},${gy}`);
    }
  }
  ctx.fillStyle = "#78716c";
  for (const key of roomDotPositions) {
    const [gxs, gys] = key.split(",");
    const gx = parseInt(gxs, 10);
    const gy = parseInt(gys, 10);
    ctx.fillRect(gx * CELL_SIZE - 1, gy * CELL_SIZE - 1, 2, 2);
  }

  // Draw cells
  for (const cell of cells.values()) {
    const px = cell.x * CELL_SIZE;
    const py = cell.y * CELL_SIZE;

    const isSelected = cell.room_id != null && (selectedRooms?.has(cell.room_id) ?? false);
    if (cell.room_id != null) {
      const hue = cell.hue ?? 40;
      if (isSelected) {
        ctx.fillStyle = `hsla(${hue}, 60%, 55%, 1)`;
      } else if (cell.is_explored) {
        ctx.fillStyle = `hsla(${hue}, 30%, 30%, 0.8)`;
      } else {
        ctx.fillStyle = `hsla(${hue}, 15%, 20%, 0.5)`;
      }
      ctx.fillRect(px, py, CELL_SIZE, CELL_SIZE);
    } else if (cell.hue != null) {
      const hue = cell.hue;
      if (isSelected) {
        ctx.fillStyle = `hsla(${hue}, 60%, 55%, 1)`;
      } else if (cell.is_explored) {
        ctx.fillStyle = `hsla(${hue}, 30%, 30%, 0.8)`;
      } else {
        ctx.fillStyle = `hsla(${hue}, 15%, 20%, 0.5)`;
      }
      ctx.fillRect(px, py, CELL_SIZE, CELL_SIZE);
    } else if (cell.is_explored) {
      ctx.fillStyle = isSelected ? "rgba(120, 113, 108, 0.6)" : "rgba(120, 113, 108, 0.3)";
      ctx.fillRect(px, py, CELL_SIZE, CELL_SIZE);
    }
  }

  // Draw walls between room cells
  ctx.lineWidth = 3;
  ctx.strokeStyle = "#d6d3d1";
  ctx.lineCap = "round";

  for (const cell of cells.values()) {
    if (cell.room_id == null) continue;
    const px = cell.x * CELL_SIZE;
    const py = cell.y * CELL_SIZE;

    // Top wall
    drawWallEdge(ctx, cells, walls, cell, cell.x, cell.y - 1, px, py, px + CELL_SIZE, py);
    // Left wall
    drawWallEdge(ctx, cells, walls, cell, cell.x - 1, cell.y, px, py, px, py + CELL_SIZE);
    // Right wall
    drawWallEdge(ctx, cells, walls, cell, cell.x + 1, cell.y, px + CELL_SIZE, py, px + CELL_SIZE, py + CELL_SIZE);
    // Bottom wall
    drawWallEdge(ctx, cells, walls, cell, cell.x, cell.y + 1, px, py + CELL_SIZE, px + CELL_SIZE, py + CELL_SIZE);
  }

  // Draw drag preview
  if (dragPreview) {
    const x = Math.min(dragPreview.x1, dragPreview.x2) * CELL_SIZE;
    const y = Math.min(dragPreview.y1, dragPreview.y2) * CELL_SIZE;
    const w = (Math.abs(dragPreview.x2 - dragPreview.x1) + 1) * CELL_SIZE;
    const h = (Math.abs(dragPreview.y2 - dragPreview.y1) + 1) * CELL_SIZE;
    ctx.strokeStyle = "rgba(251, 191, 36, 0.8)";
    ctx.lineWidth = 2;
    ctx.setLineDash([4, 4]);
    ctx.strokeRect(x, y, w, h);
    ctx.fillStyle = "rgba(251, 191, 36, 0.15)";
    ctx.fillRect(x, y, w, h);
    ctx.setLineDash([]);
  }

  // Draw hovered edge highlight (for door tool)
  if (hoveredEdge) {
    // The edge is between cell (x1,y1) and neighbor (x2,y2).
    // Find the shared wall segment in pixel coordinates.
    const dx = hoveredEdge.x2 - hoveredEdge.x1;
    const dy = hoveredEdge.y2 - hoveredEdge.y1;
    let lx1: number, ly1: number, lx2: number, ly2: number;
    if (dx === 1) {
      // neighbor is to the right: right edge of (x1,y1)
      lx1 = (hoveredEdge.x1 + 1) * CELL_SIZE;
      ly1 = hoveredEdge.y1 * CELL_SIZE;
      lx2 = lx1;
      ly2 = ly1 + CELL_SIZE;
    } else if (dx === -1) {
      // neighbor is to the left: left edge of (x1,y1)
      lx1 = hoveredEdge.x1 * CELL_SIZE;
      ly1 = hoveredEdge.y1 * CELL_SIZE;
      lx2 = lx1;
      ly2 = ly1 + CELL_SIZE;
    } else if (dy === 1) {
      // neighbor is below: bottom edge of (x1,y1)
      lx1 = hoveredEdge.x1 * CELL_SIZE;
      ly1 = (hoveredEdge.y1 + 1) * CELL_SIZE;
      lx2 = lx1 + CELL_SIZE;
      ly2 = ly1;
    } else {
      // neighbor is above: top edge of (x1,y1)
      lx1 = hoveredEdge.x1 * CELL_SIZE;
      ly1 = hoveredEdge.y1 * CELL_SIZE;
      lx2 = lx1 + CELL_SIZE;
      ly2 = ly1;
    }
    ctx.strokeStyle = hoveredEdgeValid
      ? "rgba(74, 222, 128, 0.7)"   // green — valid boundary
      : "rgba(248, 113, 113, 0.5)"; // red — not a boundary
    ctx.lineWidth = 5;
    ctx.lineCap = "round";
    ctx.beginPath();
    ctx.moveTo(lx1, ly1);
    ctx.lineTo(lx2, ly2);
    ctx.stroke();
  }

  // Draw hovered cell highlight (for letter tool)
  if (hoveredCell) {
    const px = hoveredCell.x * CELL_SIZE;
    const py = hoveredCell.y * CELL_SIZE;
    ctx.fillStyle = "rgba(255, 255, 255, 0.1)";
    ctx.fillRect(px, py, CELL_SIZE, CELL_SIZE);
  }

  ctx.restore();

  // Draw room notes in screen space (outside camera transform) for crisp text.
  // Group cells by room_id and render text at each room's centroid.
  const roomCellGroups = new Map<number, Cell[]>();
  for (const cell of cells.values()) {
    if (cell.room_id == null || !cell.text) continue;
    let group = roomCellGroups.get(cell.room_id);
    if (!group) {
      group = [];
      roomCellGroups.set(cell.room_id, group);
    }
    group.push(cell);
  }

  ctx.font = "11px sans-serif";
  ctx.textAlign = "center";
  ctx.textBaseline = "middle";
  const lineHeight = 14;
  for (const [, roomCells] of roomCellGroups) {
    const text = roomCells[0].text;
    if (!text) continue;
    // Compute centroid of the room
    let cx = 0, cy = 0;
    for (const c of roomCells) {
      cx += c.x;
      cy += c.y;
    }
    cx = cx / roomCells.length + 0.5;
    cy = cy / roomCells.length + 0.5;
    const [sx, sy] = camera.worldToScreen(cx * CELL_SIZE, cy * CELL_SIZE);
    ctx.fillStyle = "#e7e5e4";
    const lines = text.split("\n");
    const topY = sy - (lines.length - 1) * lineHeight / 2;
    for (let i = 0; i < lines.length; i++) {
      ctx.fillText(lines[i], sx, topY + i * lineHeight);
    }
  }

  // Draw markers in screen space (outside camera transform) for crisp text
  ctx.font = "bold 16px monospace";
  ctx.textAlign = "center";
  ctx.textBaseline = "middle";
  ctx.fillStyle = "#fbbf24";
  for (const marker of markers.values()) {
    const [sx, sy] = camera.worldToScreen(
      marker.x * CELL_SIZE + CELL_SIZE / 2,
      marker.y * CELL_SIZE + CELL_SIZE / 2,
    );
    ctx.fillText(marker.letter, sx, sy);
  }
}

function drawWallEdge(
  ctx: CanvasRenderingContext2D,
  cells: Map<string, Cell>,
  walls: Map<string, Wall>,
  cell: Cell,
  nx: number, ny: number,
  lx1: number, ly1: number, lx2: number, ly2: number,
) {
  const neighbor = cells.get(cellKey(nx, ny));
  const wKey = wallKey(cell.x, cell.y, nx, ny);
  const wall = walls.get(wKey);

  if (wall?.type === "open") {
    return;
  }

  if (wall?.type === "door") {
    const mx = (lx1 + lx2) / 2;
    const my = (ly1 + ly2) / 2;
    const gap = CELL_SIZE * 0.3;

    ctx.strokeStyle = "#d6d3d1";
    ctx.lineWidth = 3;

    if (lx1 === lx2) {
      ctx.beginPath();
      ctx.moveTo(lx1, ly1);
      ctx.lineTo(lx1, my - gap);
      ctx.stroke();
      ctx.beginPath();
      ctx.moveTo(lx1, my + gap);
      ctx.lineTo(lx1, ly2);
      ctx.stroke();
      ctx.lineWidth = 2;
      ctx.beginPath();
      ctx.moveTo(lx1 - 4, my - gap);
      ctx.lineTo(lx1 + 4, my - gap);
      ctx.stroke();
      ctx.beginPath();
      ctx.moveTo(lx1 - 4, my + gap);
      ctx.lineTo(lx1 + 4, my + gap);
      ctx.stroke();
    } else {
      ctx.beginPath();
      ctx.moveTo(lx1, ly1);
      ctx.lineTo(mx - gap, ly1);
      ctx.stroke();
      ctx.beginPath();
      ctx.moveTo(mx + gap, ly1);
      ctx.lineTo(lx2, ly1);
      ctx.stroke();
      ctx.lineWidth = 2;
      ctx.beginPath();
      ctx.moveTo(mx - gap, ly1 - 4);
      ctx.lineTo(mx - gap, ly1 + 4);
      ctx.stroke();
      ctx.beginPath();
      ctx.moveTo(mx + gap, ly1 - 4);
      ctx.lineTo(mx + gap, ly1 + 4);
      ctx.stroke();
    }
    return;
  }

  // Default: solid wall if neighbor is not in the same room
  if (neighbor?.room_id === cell.room_id && neighbor?.room_id != null) {
    return;
  }

  ctx.strokeStyle = "#d6d3d1";
  ctx.lineWidth = 3;
  ctx.beginPath();
  ctx.moveTo(lx1, ly1);
  ctx.lineTo(lx2, ly2);
  ctx.stroke();
}

// --- Hex Grid (Hex Mode) ---

export const HEX_SIZE = 30;

function hexCorner(cx: number, cy: number, i: number): [number, number] {
  const angle = (Math.PI / 180) * (60 * i);
  return [cx + HEX_SIZE * Math.cos(angle), cy + HEX_SIZE * Math.sin(angle)];
}

export function hexToPixel(col: number, row: number): [number, number] {
  const x = HEX_SIZE * 1.5 * col;
  const y = HEX_SIZE * Math.sqrt(3) * (row + 0.5 * (col & 1));
  return [x, y];
}

export function pixelToHex(px: number, py: number): [number, number] {
  // Flat-top: pixel to fractional axial coordinates.
  const q = (px * 2 / 3) / HEX_SIZE;
  const r = (-px / 3 + py * Math.sqrt(3) / 3) / HEX_SIZE;

  // Convert axial (q, r) to cube (x, y, z)
  let cx = q;
  let cz = r;
  let cy = -cx - cz;

  // Round cube coordinates
  let rx = Math.round(cx);
  let ry = Math.round(cy);
  let rz = Math.round(cz);

  const xDiff = Math.abs(rx - cx);
  const yDiff = Math.abs(ry - cy);
  const zDiff = Math.abs(rz - cz);

  if (xDiff > yDiff && xDiff > zDiff) {
    rx = -ry - rz;
  } else if (yDiff > zDiff) {
    ry = -rx - rz;
  } else {
    rz = -rx - ry;
  }

  // Axial to odd-q offset: col = q, row = r + (q - (q&1)) / 2
  const offsetCol = rx;
  const offsetRow = rz + (rx - (rx & 1)) / 2;

  return [offsetCol, offsetRow];
}

function drawHexOutline(ctx: CanvasRenderingContext2D, cx: number, cy: number) {
  ctx.beginPath();
  for (let i = 0; i < 6; i++) {
    const [hx, hy] = hexCorner(cx, cy, i);
    if (i === 0) ctx.moveTo(hx, hy);
    else ctx.lineTo(hx, hy);
  }
  ctx.closePath();
}

export function drawHexGrid(
  ctx: CanvasRenderingContext2D,
  camera: Camera,
  cells: Map<string, Cell>,
  markers: Map<string, Marker>,
  selection: Set<string> | null,
) {
  ctx.save();
  camera.apply(ctx);

  // Determine visible range
  const [tlx, tly] = camera.screenToWorld(0, 0);
  const [brx, bry] = camera.screenToWorld(camera.logicalWidth, camera.logicalHeight);
  const hexW = HEX_SIZE * 1.5;
  const hexH = HEX_SIZE * Math.sqrt(3);
  const minCol = Math.floor(tlx / hexW) - 2;
  const maxCol = Math.ceil(brx / hexW) + 2;
  const minRow = Math.floor(tly / hexH) - 2;
  const maxRow = Math.ceil(bry / hexH) + 2;

  // Draw hex grid outlines for hexes that have no visible fill
  ctx.strokeStyle = "#44403c";
  ctx.lineWidth = 1;
  for (let row = minRow; row <= maxRow; row++) {
    for (let col = minCol; col <= maxCol; col++) {
      const cell = cells.get(cellKey(col, row));
      if (cell && cellHasData(cell)) continue;
      const key = cellKey(col, row);
      const [hx, hy] = hexToPixel(col, row);
      drawHexOutline(ctx, hx, hy);
      if (selection?.has(key)) {
        ctx.fillStyle = "rgba(255, 255, 255, 0.1)";
        ctx.fill();
        ctx.strokeStyle = "#fbbf24";
        ctx.lineWidth = 2;
        ctx.stroke();
        ctx.strokeStyle = "#44403c";
        ctx.lineWidth = 1;
      } else {
        ctx.stroke();
      }
    }
  }

  // Draw cells that have meaningful data
  for (const cell of cells.values()) {
    if (!cellHasData(cell)) continue;
    const [hx, hy] = hexToPixel(cell.x, cell.y);
    const hue = cell.hue ?? 40;
    const isSelected = selection?.has(cellKey(cell.x, cell.y));

    if (isSelected) {
      ctx.fillStyle = `hsla(${hue}, 60%, 55%, 1)`;
    } else if (cell.is_explored) {
      ctx.fillStyle = `hsla(${hue}, 30%, 30%, 0.8)`;
    } else {
      ctx.fillStyle = `hsla(${hue}, 15%, 20%, 0.5)`;
    }
    drawHexOutline(ctx, hx, hy);
    ctx.fill();
    ctx.strokeStyle = cell.is_explored ? "#78716c" : "#57534e";
    ctx.lineWidth = 1;
    ctx.stroke();

  }

  ctx.restore();

  // Draw text in screen space (outside camera transform) for crisp rendering
  ctx.font = "11px sans-serif";
  ctx.textAlign = "center";
  ctx.textBaseline = "middle";
  const lineHeight = 14;
  for (const cell of cells.values()) {
    if (!cellHasData(cell) || !cell.text) continue;
    const [hx, hy] = hexToPixel(cell.x, cell.y);
    const [sx, sy] = camera.worldToScreen(hx, hy);
    ctx.fillStyle = cell.is_explored ? "#e7e5e4" : "#a8a29e";
    const lines = cell.text.split("\n");
    const topY = sy - (lines.length - 1) * lineHeight / 2;
    for (let i = 0; i < lines.length; i++) {
      ctx.fillText(lines[i], sx, topY + i * lineHeight);
    }
  }

  // Draw markers in screen space
  ctx.font = "bold 16px monospace";
  ctx.textAlign = "center";
  ctx.textBaseline = "middle";
  ctx.fillStyle = "#fbbf24";
  for (const marker of markers.values()) {
    const [hx, hy] = hexToPixel(marker.x, marker.y);
    const [sx, sy] = camera.worldToScreen(hx, hy);
    ctx.fillText(marker.letter, sx, sy);
  }
}

// --- Shared Utilities ---

// A cell "has data" if any property has been set beyond the default empty state.
// Cells created on-demand just for the properties panel should be invisible.
export function cellHasData(cell: Cell): boolean {
  return cell.is_explored || cell.hue != null || cell.text !== "" || cell.room_id != null;
}

export function cellKey(x: number, y: number): string {
  return `${x},${y}`;
}

export function wallKey(x1: number, y1: number, x2: number, y2: number): string {
  if (x1 > x2 || (x1 === x2 && y1 > y2)) {
    [x1, y1, x2, y2] = [x2, y2, x1, y1];
  }
  return `${x1},${y1}-${x2},${y2}`;
}

export function nearestWallEdge(
  worldX: number, worldY: number,
): { cellX: number; cellY: number; neighborX: number; neighborY: number } | null {
  const gx = Math.floor(worldX / CELL_SIZE);
  const gy = Math.floor(worldY / CELL_SIZE);
  const fx = (worldX / CELL_SIZE) - gx;
  const fy = (worldY / CELL_SIZE) - gy;

  const distTop = fy;
  const distBottom = 1 - fy;
  const distLeft = fx;
  const distRight = 1 - fx;
  const minDist = Math.min(distTop, distBottom, distLeft, distRight);

  if (minDist > 0.35) return null;

  if (minDist === distTop) return { cellX: gx, cellY: gy, neighborX: gx, neighborY: gy - 1 };
  if (minDist === distBottom) return { cellX: gx, cellY: gy, neighborX: gx, neighborY: gy + 1 };
  if (minDist === distLeft) return { cellX: gx, cellY: gy, neighborX: gx - 1, neighborY: gy };
  return { cellX: gx, cellY: gy, neighborX: gx + 1, neighborY: gy };
}
