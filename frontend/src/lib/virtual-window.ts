export interface VirtualWindow {
  start: number;
  end: number;
  padTop: number;
  padBottom: number;
}

export const VIRTUALIZE_THRESHOLD = 200;

export function computeVirtualWindow(
  total: number,
  scrollTop: number,
  viewportHeight: number,
  rowHeight: number,
  overscan = 8,
): VirtualWindow {
  if (total <= 0 || rowHeight <= 0) {
    return { start: 0, end: 0, padTop: 0, padBottom: 0 };
  }
  if (total <= VIRTUALIZE_THRESHOLD) {
    return { start: 0, end: total, padTop: 0, padBottom: 0 };
  }
  const visible = Math.max(1, Math.ceil(Math.max(viewportHeight, rowHeight) / rowHeight));
  const maxStart = Math.max(0, total - visible);
  const first = Math.min(maxStart, Math.max(0, Math.floor(Math.max(scrollTop, 0) / rowHeight)));
  const start = Math.max(0, first - overscan);
  const end = Math.min(total, first + visible + overscan);
  return { start, end, padTop: start * rowHeight, padBottom: (total - end) * rowHeight };
}
