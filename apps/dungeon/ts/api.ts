import { Cell, Wall, Marker, MapState } from "./types";

function basePath(): string {
  // Works whether accessed via /dungeon/ or directly
  const path = window.location.pathname;
  const match = path.match(/^(.*\/maps\/\d+)\//);
  if (match) {
    return match[1];
  }
  return path.replace(/\/$/, "");
}

export async function fetchMapState(mapID: number): Promise<MapState> {
  const resp = await fetch(`${basePath()}/state/`);
  return resp.json();
}

export async function upsertCells(mapID: number, cells: Partial<Cell>[]): Promise<Cell[]> {
  const resp = await fetch(`${basePath()}/cells/`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(cells),
  });
  return resp.json();
}

export async function upsertWall(mapID: number, wall: Partial<Wall>): Promise<Wall> {
  const resp = await fetch(`${basePath()}/walls/`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(wall),
  });
  return resp.json();
}

export async function upsertMarker(mapID: number, marker: Partial<Marker>): Promise<Marker> {
  const resp = await fetch(`${basePath()}/markers/`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(marker),
  });
  return resp.json();
}

export async function deleteMarker(mapID: number, x: number, y: number): Promise<void> {
  await fetch(`${basePath()}/markers/delete/`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ x, y }),
  });
}
