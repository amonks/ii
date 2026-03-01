export class Camera {
  x = 0;
  y = 0;
  zoom = 4;

  // Logical (CSS pixel) dimensions, updated on resize
  logicalWidth = 0;
  logicalHeight = 0;

  private minZoom = 0.25;

  pan(dx: number, dy: number) {
    this.x += dx / this.zoom;
    this.y += dy / this.zoom;
  }

  zoomAt(factor: number, screenX: number, screenY: number) {
    const newZoom = Math.max(this.minZoom, this.zoom * factor);
    const ratio = newZoom / this.zoom;

    // Adjust pan so zoom centers on the pointer position
    const cx = this.logicalWidth / 2;
    const cy = this.logicalHeight / 2;
    this.x -= (screenX - cx) * (1 - 1 / ratio) / newZoom;
    this.y -= (screenY - cy) * (1 - 1 / ratio) / newZoom;

    this.zoom = newZoom;
  }

  // Convert screen (CSS pixel) coordinates to world coordinates
  screenToWorld(sx: number, sy: number): [number, number] {
    const cx = this.logicalWidth / 2;
    const cy = this.logicalHeight / 2;
    const wx = (sx - cx) / this.zoom - this.x;
    const wy = (sy - cy) / this.zoom - this.y;
    return [wx, wy];
  }

  // Convert world coordinates to screen (CSS pixel) coordinates
  worldToScreen(wx: number, wy: number): [number, number] {
    const cx = this.logicalWidth / 2;
    const cy = this.logicalHeight / 2;
    const sx = (wx + this.x) * this.zoom + cx;
    const sy = (wy + this.y) * this.zoom + cy;
    return [sx, sy];
  }

  // Apply camera transform to canvas context.
  // Call this AFTER the DPR scale has been applied to the context.
  apply(ctx: CanvasRenderingContext2D) {
    const cx = this.logicalWidth / 2;
    const cy = this.logicalHeight / 2;
    ctx.translate(cx, cy);
    ctx.scale(this.zoom, this.zoom);
    ctx.translate(this.x, this.y);
  }
}
