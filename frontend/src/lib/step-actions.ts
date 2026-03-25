export const supportedStepActions = [
  'navigate', 'click', 'type', 'wait', 'screenshot', 'extract', 'scroll', 'select',
  'if_element', 'if_text', 'if_url', 'loop', 'end_loop', 'break_loop', 'goto', 'solve_captcha', 'click_ad',
  'while_condition', 'end_while', 'if_exists', 'if_not_exists', 'if_visible', 'if_enabled',
  'variable_set', 'variable_math', 'variable_string',
  'hover', 'drag_drop', 'context_click', 'highlight', 'select_random',
  'get_cookies', 'set_cookie', 'delete_cookies', 'get_storage', 'set_storage', 'delete_storage',
  'download', 'debug_pause', 'debug_resume', 'debug_step',
  'double_click', 'file_upload', 'navigate_back', 'navigate_forward', 'reload',
  'scroll_into_view', 'submit_form', 'wait_not_present', 'wait_enabled', 'wait_function',
  'emulate_device', 'get_title', 'get_attributes',
  'anti_bot', 'random_mouse', 'human_typing',
  'get_session', 'set_session', 'load_session', 'save_session',
  'cache_get', 'cache_set', 'cache_clear'
] as const;

import { get, writable } from 'svelte/store';

export type SupportedStepAction = typeof supportedStepActions[number];

export interface StepActionState {
  actions: string[];
  usingFallback: boolean;
  warning: string;
  loaded: boolean;
}

type WailsAppMethods = {
  GetSupportedStepActions?: () => Promise<string[]>;
};

const fallbackWarning = 'Using bundled step actions because the backend action catalog could not be loaded.';

export const stepActionState = writable<StepActionState>({
  actions: [...supportedStepActions],
  usingFallback: false,
  warning: '',
  loaded: false,
});

let loadPromise: Promise<StepActionState> | null = null;

function getWailsApp(): WailsAppMethods | undefined {
  return (window as Window & {
    go?: {
      main?: {
        App?: WailsAppMethods;
      };
    };
  }).go?.main?.App;
}

function normalizeActions(actions: unknown): string[] {
  if (!Array.isArray(actions)) {
    return [];
  }
  return actions.filter((action): action is string => typeof action === 'string' && action.length > 0);
}

function createFallbackState(): StepActionState {
  return {
    actions: [...supportedStepActions],
    usingFallback: true,
    warning: fallbackWarning,
    loaded: true,
  };
}

export async function loadSupportedStepActions(): Promise<string[]> {
  const app = getWailsApp();
  const actions = normalizeActions(await app?.GetSupportedStepActions?.());
  if (actions.length === 0) {
    return [...supportedStepActions];
  }
  return actions;
}

async function resolveStepActionState(): Promise<StepActionState> {
  const app = getWailsApp();
  if (!app?.GetSupportedStepActions) {
    return createFallbackState();
  }
  const actions = normalizeActions(await app.GetSupportedStepActions());
  if (actions.length === 0) {
    return createFallbackState();
  }
  return {
    actions,
    usingFallback: false,
    warning: '',
    loaded: true,
  };
}

export function getStepActionOptions(currentAction: string, actions: string[]): string[] {
  if (!currentAction || actions.includes(currentAction)) {
    return actions;
  }
  return [...actions, currentAction];
}

export function ensureStepActionStateLoaded(): Promise<StepActionState> {
  if (get(stepActionState).loaded) {
    return Promise.resolve(get(stepActionState));
  }
  if (loadPromise) {
    return loadPromise;
  }
  loadPromise = resolveStepActionState()
    .then((next) => {
      stepActionState.set(next);
      return next;
    })
    .catch(() => {
      const next = createFallbackState();
      stepActionState.set(next);
      return next;
    })
    .finally(() => {
      loadPromise = null;
    });
  return loadPromise;
}

export function resetStepActionStateForTest(): void {
  loadPromise = null;
  stepActionState.set({
    actions: [...supportedStepActions],
    usingFallback: false,
    warning: '',
    loaded: false,
  });
}
