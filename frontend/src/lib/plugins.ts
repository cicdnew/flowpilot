import type { PluginInfo, PluginStatus } from './types';

export function normalizePluginStatus(raw: unknown): PluginStatus {
  const src = (raw ?? {}) as Partial<PluginStatus>;
  const plugins = (Array.isArray(src.plugins) ? src.plugins : []).map((p: Partial<PluginInfo>) => ({
    ...p,
    actions: Array.isArray(p.actions) ? p.actions : [],
    hooks: Array.isArray(p.hooks) ? p.hooks : [],
    capabilities: Array.isArray(p.capabilities) ? p.capabilities : [],
  })) as PluginInfo[];
  return {
    dir: typeof src.dir === 'string' ? src.dir : '',
    plugins,
    errors: Array.isArray(src.errors) ? src.errors.filter((e) => typeof e === 'string') : [],
  };
}

export function pluginActionCount(status: PluginStatus): number {
  return status.plugins.reduce((total, p) => total + p.actions.length, 0);
}
