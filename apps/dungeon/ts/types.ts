export interface MapState {
  map: DungeonMap;
  cells: Cell[];
  walls: Wall[];
  markers: Marker[];
}

export interface DungeonMap {
  id: number;
  name: string;
  type: "dungeon" | "hex";
}

export interface Cell {
  id?: number;
  map_id: number;
  x: number;
  y: number;
  is_explored: boolean;
  text: string;
  hue: number | null;
  room_id: number | null;
}

export interface Wall {
  id?: number;
  map_id: number;
  x1: number;
  y1: number;
  x2: number;
  y2: number;
  type: "open" | "door";
}

export interface Marker {
  id?: number;
  map_id: number;
  x: number;
  y: number;
  letter: string;
}

export interface SSEEvent {
  type: string;
  data: any;
}

export type ToolName = "select" | "box" | "door" | "letter" | "paint" | "subtract";
