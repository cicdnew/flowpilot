import { beforeEach, describe, expect, it } from 'vitest';
import { get } from 'svelte/store';

import {
  ensureStepActionStateLoaded,
  getStepActionOptions,
  resetStepActionStateForTest,
  stepActionState,
  supportedStepActions,
} from './step-actions';

declare global {
  interface Window {
    go?: {
      main?: {
        App?: {
          GetSupportedStepActions?: () => Promise<string[]>;
        };
      };
    };
  }
}

describe('step-actions', () => {
  beforeEach(() => {
    resetStepActionStateForTest();
    delete window.go;
  });

  it('falls back with warning when backend action catalog is unavailable', async () => {
    const state = await ensureStepActionStateLoaded();

    expect(state.usingFallback).toBe(true);
    expect(state.warning).not.toBe('');
    expect(state.actions).toEqual([...supportedStepActions]);
    expect(get(stepActionState)).toEqual(state);
  });

  it('loads backend-supported actions when available', async () => {
    window.go = {
      main: {
        App: {
          GetSupportedStepActions: async () => ['navigate', 'click', 'custom_action'],
        },
      },
    };

    const state = await ensureStepActionStateLoaded();

    expect(state.usingFallback).toBe(false);
    expect(state.warning).toBe('');
    expect(state.actions).toEqual(['navigate', 'click', 'custom_action']);
  });

  it('preserves unknown current action in options', () => {
    expect(getStepActionOptions('legacy_action', ['navigate', 'click'])).toEqual(['navigate', 'click', 'legacy_action']);
    expect(getStepActionOptions('click', ['navigate', 'click'])).toEqual(['navigate', 'click']);
  });
});
