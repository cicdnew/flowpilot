import { describe, expect, it } from 'vitest';
import { normalizePluginStatus, pluginActionCount } from './plugins';

describe('normalizePluginStatus', () => {
  it('handles null slices from Go', () => {
    const status = normalizePluginStatus({ dir: '/data/plugins', plugins: null, errors: null });
    expect(status).toEqual({ dir: '/data/plugins', plugins: [], errors: [] });
  });

  it('fills missing plugin lists and counts actions', () => {
    const status = normalizePluginStatus({
      dir: '/p',
      plugins: [
        { id: 'a', name: 'A', version: '1', type: 'process', actions: [{ name: 'x' }, { name: 'y' }], hooks: null, capabilities: null, loadedAt: '' },
        { id: 'b', name: 'B', version: '1', type: 'process', actions: null, hooks: ['pre_execute'], loadedAt: '' },
      ],
      errors: ['bad manifest', 42],
    });
    expect(status.plugins[0].hooks).toEqual([]);
    expect(status.plugins[0].capabilities).toEqual([]);
    expect(status.plugins[1].actions).toEqual([]);
    expect(status.plugins[1].hooks).toEqual(['pre_execute']);
    expect(status.errors).toEqual(['bad manifest']);
    expect(pluginActionCount(status)).toBe(2);
  });

  it('tolerates undefined input', () => {
    expect(normalizePluginStatus(undefined)).toEqual({ dir: '', plugins: [], errors: [] });
  });
});
