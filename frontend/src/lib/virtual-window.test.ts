import { describe, expect, it } from 'vitest';
import { computeVirtualWindow, VIRTUALIZE_THRESHOLD } from './virtual-window';

describe('computeVirtualWindow', () => {
  it('renders everything for small lists', () => {
    expect(computeVirtualWindow(50, 0, 600, 44)).toEqual({ start: 0, end: 50, padTop: 0, padBottom: 0 });
    expect(computeVirtualWindow(0, 0, 600, 44)).toEqual({ start: 0, end: 0, padTop: 0, padBottom: 0 });
  });

  it('keeps a bounded number of rows in the DOM for 10,000 tasks', () => {
    const total = 10000;
    const rowHeight = 44;
    for (const scrollTop of [0, 1234, 220000, total * rowHeight]) {
      const w = computeVirtualWindow(total, scrollTop, 600, rowHeight);
      expect(w.end - w.start).toBeLessThanOrEqual(Math.ceil(600 / rowHeight) + 16);
      expect(w.padTop + (w.end - w.start) * rowHeight + w.padBottom).toBe(total * rowHeight);
      expect(w.start).toBeGreaterThanOrEqual(0);
      expect(w.end).toBeLessThanOrEqual(total);
    }
  });

  it('shows the rows at the scroll position', () => {
    const w = computeVirtualWindow(10000, 44 * 5000, 440, 44, 2);
    expect(w.start).toBe(4998);
    expect(w.end).toBe(5012);
  });

  it('clamps over-scrolling to the last page', () => {
    const w = computeVirtualWindow(VIRTUALIZE_THRESHOLD + 100, 1e9, 440, 44, 0);
    expect(w.end).toBe(VIRTUALIZE_THRESHOLD + 100);
    expect(w.padBottom).toBe(0);
  });

  it('computes 10,000-row windows fast enough for 60 fps scrolling', () => {
    const start = performance.now();
    for (let i = 0; i < 10000; i++) computeVirtualWindow(10000, i * 44, 600, 44);
    expect((performance.now() - start) / 10000).toBeLessThan(1);
  });
});
