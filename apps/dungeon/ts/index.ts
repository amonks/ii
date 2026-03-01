import { Cell, Wall, Marker, ToolName } from "./types";
import { Camera } from "./camera";
import {
  drawDungeonGrid, drawHexGrid,
  cellKey, wallKey,
} from "./grid";
import { TOOLS, AppState, Tool } from "./tools";
import { connectSSE } from "./sse";
import { fetchMapState } from "./api";
import { showProperties, hideProperties } from "./panel";

function init() {
  const appEl = document.getElementById("app");
  if (!appEl) return;

  const mapID = parseInt(appEl.dataset.mapId!, 10);
  const mapType = appEl.dataset.mapType as "dungeon" | "hex";
  const canvas = document.getElementById("canvas") as HTMLCanvasElement;
  const ctx = canvas.getContext("2d")!;
  const camera = new Camera();

  // State
  const cells = new Map<string, Cell>();
  const walls = new Map<string, Wall>();
  const markers = new Map<string, Marker>();
  const selectedRooms = new Set<number>();
  const selectedHexes = new Set<string>();

  let currentTool: Tool = TOOLS.select;
  let renderRequested = false;

  const state: AppState = {
    mapID,
    mapType,
    cells,
    walls,
    markers,
    selectedRooms,
    selectedHexes,
    shiftDown: false,
    hoveredEdge: null,
    hoveredEdgeValid: false,
    hoveredCell: null,
    camera,
    canvas,
    dragPreview: null,
    requestRender() {
      if (!renderRequested) {
        renderRequested = true;
        requestAnimationFrame(render);
      }
    },
    showProperties(selectedCells: Cell[]) {
      showProperties(state, selectedCells);
    },
    hideProperties() {
      hideProperties();
    },
  };

  // --- Rendering ---

  function resizeCanvas() {
    const dpr = window.devicePixelRatio || 1;
    const rect = canvas.getBoundingClientRect();
    canvas.width = rect.width * dpr;
    canvas.height = rect.height * dpr;
    // Store logical (CSS pixel) dimensions on the camera
    camera.logicalWidth = rect.width;
    camera.logicalHeight = rect.height;
    state.requestRender();
  }

  function render() {
    renderRequested = false;
    const dpr = window.devicePixelRatio || 1;
    // Reset transform and apply DPR scale — all subsequent drawing
    // is in CSS pixel coordinates, scaled up for sharpness.
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.clearRect(0, 0, camera.logicalWidth, camera.logicalHeight);

    if (mapType === "dungeon") {
      drawDungeonGrid(ctx, camera, cells, walls, markers, selectedRooms, state.dragPreview, state.hoveredEdge, state.hoveredEdgeValid, state.hoveredCell);
    } else {
      drawHexGrid(ctx, camera, cells, markers, selectedHexes);
    }
  }

  // --- Input Handling ---

  let pointerDown = false;
  let isPanning = false;
  let lastPointerX = 0;
  let lastPointerY = 0;

  // Track touches for pinch-zoom
  const activeTouches = new Map<number, { x: number; y: number }>();

  // Convert a pointer event to CSS pixel coordinates relative to the canvas.
  function canvasCoords(e: { clientX: number; clientY: number }): [number, number] {
    const rect = canvas.getBoundingClientRect();
    return [e.clientX - rect.left, e.clientY - rect.top];
  }

  canvas.addEventListener("pointerdown", (e) => {
    if (e.pointerType === "touch") {
      activeTouches.set(e.pointerId, { x: e.clientX, y: e.clientY });
      if (activeTouches.size >= 2) {
        isPanning = true;
        return;
      }
    }

    pointerDown = true;
    lastPointerX = e.clientX;
    lastPointerY = e.clientY;

    if (e.pointerType !== "touch" && (e.button === 1 || e.button === 2)) {
      isPanning = true;
      return;
    }

    state.shiftDown = e.shiftKey;
    const [sx, sy] = canvasCoords(e);
    const [wx, wy] = camera.screenToWorld(sx, sy);
    currentTool.onPointerDown(state, wx, wy);
  });

  canvas.addEventListener("pointermove", (e) => {
    if (e.pointerType === "touch") {
      activeTouches.set(e.pointerId, { x: e.clientX, y: e.clientY });

      if (activeTouches.size >= 2 && isPanning) {
        const touches = Array.from(activeTouches.values());
        const cx = touches.reduce((s, t) => s + t.x, 0) / touches.length;
        const cy = touches.reduce((s, t) => s + t.y, 0) / touches.length;
        if (lastPointerX !== 0 || lastPointerY !== 0) {
          camera.pan(cx - lastPointerX, cy - lastPointerY);
          state.requestRender();
        }
        lastPointerX = cx;
        lastPointerY = cy;
        return;
      }
    }

    if (isPanning && pointerDown) {
      camera.pan(e.clientX - lastPointerX, e.clientY - lastPointerY);
      lastPointerX = e.clientX;
      lastPointerY = e.clientY;
      state.requestRender();
      return;
    }

    const [sx, sy] = canvasCoords(e);
    const [wx, wy] = camera.screenToWorld(sx, sy);
    currentTool.onPointerMove(state, wx, wy);
  });

  const pointerUpHandler = (e: PointerEvent) => {
    if (e.pointerType === "touch") {
      activeTouches.delete(e.pointerId);
      if (activeTouches.size < 2) {
        isPanning = false;
      }
      if (activeTouches.size === 0) {
        pointerDown = false;
      }
      return;
    }

    if (isPanning) {
      isPanning = false;
      pointerDown = false;
      return;
    }

    if (!pointerDown) return;
    pointerDown = false;

    const [sx, sy] = canvasCoords(e);
    const [wx, wy] = camera.screenToWorld(sx, sy);
    currentTool.onPointerUp(state, wx, wy);
  };

  canvas.addEventListener("pointerup", pointerUpHandler);
  canvas.addEventListener("pointercancel", pointerUpHandler);

  // Pinch zoom
  let lastPinchDist = 0;

  canvas.addEventListener("touchmove", (e) => {
    if (e.touches.length === 2) {
      e.preventDefault();
      const t1 = e.touches[0];
      const t2 = e.touches[1];
      const dist = Math.hypot(t2.clientX - t1.clientX, t2.clientY - t1.clientY);
      if (lastPinchDist > 0) {
        const factor = dist / lastPinchDist;
        const mx = (t1.clientX + t2.clientX) / 2;
        const my = (t1.clientY + t2.clientY) / 2;
        const rect = canvas.getBoundingClientRect();
        camera.zoomAt(factor, mx - rect.left, my - rect.top);
        state.requestRender();
      }
      lastPinchDist = dist;
    }
  }, { passive: false });

  canvas.addEventListener("touchend", () => {
    lastPinchDist = 0;
  });

  // Mouse wheel: pan by default, ctrl+wheel to zoom
  canvas.addEventListener("wheel", (e) => {
    e.preventDefault();
    if (e.ctrlKey || e.metaKey) {
      // Zoom
      const factor = e.deltaY > 0 ? 0.9 : 1.1;
      const [sx, sy] = canvasCoords(e);
      camera.zoomAt(factor, sx, sy);
    } else {
      // Pan
      camera.pan(-e.deltaX, -e.deltaY);
    }
    state.requestRender();
  }, { passive: false });

  // Prevent context menu
  canvas.addEventListener("contextmenu", (e) => e.preventDefault());

  // --- Tool Bar ---

  document.querySelectorAll(".tool-btn[data-tool]").forEach((btn) => {
    btn.addEventListener("click", () => {
      const toolName = (btn as HTMLElement).dataset.tool as ToolName;
      if (!TOOLS[toolName]) return;
      currentTool = TOOLS[toolName];
      state.hoveredEdge = null;
      state.hoveredEdgeValid = false;
      state.hoveredCell = null;
      document.querySelectorAll(".tool-btn[data-tool]").forEach((b) => b.classList.remove("active"));
      btn.classList.add("active");
      state.requestRender();
    });
  });

  // Properties close button
  document.getElementById("prop-close")?.addEventListener("click", () => {
    selectedRooms.clear();
    selectedHexes.clear();
    hideProperties();
    state.requestRender();
  });

  // --- SSE ---

  connectSSE(mapID, (event) => {
    switch (event.type) {
      case "cells":
        for (const cell of event.data as Cell[]) {
          cells.set(cellKey(cell.x, cell.y), cell);
        }
        break;
      case "wall": {
        const wall = event.data as Wall;
        walls.set(wallKey(wall.x1, wall.y1, wall.x2, wall.y2), wall);
        break;
      }
      case "marker": {
        const marker = event.data as Marker;
        markers.set(cellKey(marker.x, marker.y), marker);
        break;
      }
      case "marker_delete": {
        const { x, y } = event.data as { x: number; y: number };
        markers.delete(cellKey(x, y));
        break;
      }
      case "cells_delete": {
        const deleted = event.data as { x: number; y: number }[];
        for (const { x, y } of deleted) {
          cells.delete(cellKey(x, y));
        }
        break;
      }
      case "walls_delete": {
        const deleted = event.data as Wall[];
        for (const w of deleted) {
          walls.delete(wallKey(w.x1, w.y1, w.x2, w.y2));
        }
        break;
      }
    }
    state.requestRender();
  });

  // --- Load initial state ---

  fetchMapState(mapID).then((mapState) => {
    for (const cell of mapState.cells) {
      cells.set(cellKey(cell.x, cell.y), cell);
    }
    for (const wall of mapState.walls) {
      walls.set(wallKey(wall.x1, wall.y1, wall.x2, wall.y2), wall);
    }
    for (const marker of mapState.markers) {
      markers.set(cellKey(marker.x, marker.y), marker);
    }
    state.requestRender();
  });

  // --- Resize & Initial Render ---

  window.addEventListener("resize", resizeCanvas);
  resizeCanvas();
}

if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", init);
} else {
  init();
}
