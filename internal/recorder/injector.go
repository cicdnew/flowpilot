package recorder

import (
	"encoding/json"
	"fmt"

	"flowpilot/internal/models"
)

const bindingName = "__recordStep"

const captureScript = `(function() {
  if (window.__recorderInjected) return;
  window.__recorderInjected = true;

  var debounceTimers = {};

  function getSelector(el) {
    if (!el || el === document || el === document.body) return '';
    if (el.getAttribute && el.getAttribute('data-testid'))
      return '[data-testid="' + el.getAttribute('data-testid') + '"]';
    if (el.id) return '#' + CSS.escape(el.id);
    if (el.getAttribute && el.getAttribute('name') && el.tagName)
      return el.tagName.toLowerCase() + '[name="' + el.getAttribute('name') + '"]';
    if (el.className && typeof el.className === 'string' && el.className.trim()) {
      var classes = el.className.trim().split(/\s+/).map(function(c) { return '.' + CSS.escape(c); }).join('');
      var sel = el.tagName.toLowerCase() + classes;
      if (document.querySelectorAll(sel).length === 1) return sel;
    }
    var path = [];
    var current = el;
    while (current && current !== document.body && current !== document) {
      var tag = current.tagName.toLowerCase();
      var parent = current.parentElement;
      if (parent) {
        var siblings = Array.from(parent.children).filter(function(c) { return c.tagName === current.tagName; });
        if (siblings.length > 1) {
          var idx = siblings.indexOf(current) + 1;
          tag += ':nth-of-type(' + idx + ')';
        }
      }
      path.unshift(tag);
      current = parent;
    }
    return path.join(' > ');
  }

  function isInteractive(el) {
    if (!el || !el.tagName) return false;
    var tag = el.tagName.toLowerCase();
    if (tag === 'a' || tag === 'button' || tag === 'select' || tag === 'textarea') return true;
    if (tag === 'input') return true;
    if (el.getAttribute && (el.getAttribute('role') === 'button' || el.getAttribute('onclick'))) return true;
    if (el.isContentEditable) return true;
    return false;
  }

  function findInteractive(el) {
    var current = el;
    for (var i = 0; i < 5 && current && current !== document.body; i++) {
      if (isInteractive(current)) return current;
      current = current.parentElement;
    }
    return el;
  }

  function emit(action, selector, value) {
    if (typeof window.__recordStep === 'function') {
      window.__recordStep(JSON.stringify({ action: action, selector: selector, value: value || '' }));
    }
  }

  document.addEventListener('click', function(e) {
    var target = findInteractive(e.target);
    var selector = getSelector(target);
    if (!selector) return;
    var tag = target.tagName ? target.tagName.toLowerCase() : '';
    if (tag === 'input' || tag === 'textarea' || tag === 'select') return;
    emit('click', selector, '');
  }, true);

  document.addEventListener('change', function(e) {
    var target = e.target;
    var selector = getSelector(target);
    if (!selector) return;
    var tag = target.tagName ? target.tagName.toLowerCase() : '';
    if (tag === 'select') {
      emit('select', selector, target.value || '');
      return;
    }
    emit('type', selector, target.value || '');
  }, true);

  document.addEventListener('input', function(e) {
    var target = e.target;
    var selector = getSelector(target);
    if (!selector) return;
    if (debounceTimers[selector]) clearTimeout(debounceTimers[selector]);
    debounceTimers[selector] = setTimeout(function() {
      delete debounceTimers[selector];
      emit('type', selector, target.value || '');
    }, 500);
  }, true);
})();`

type bindingPayload struct {
	Action   string `json:"action"`
	Selector string `json:"selector"`
	Value    string `json:"value"`
}

func parseBindingPayload(raw string) (models.StepAction, string, string, error) {
	var p bindingPayload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return "", "", "", fmt.Errorf("parse binding payload: %w", err)
	}
	action, ok := mapJSAction(p.Action)
	if !ok {
		return "", "", "", fmt.Errorf("unknown action from browser: %s", p.Action)
	}
	return action, p.Selector, p.Value, nil
}

func mapJSAction(jsAction string) (models.StepAction, bool) {
	switch jsAction {
	case "click":
		return models.ActionClick, true
	case "type":
		return models.ActionType, true
	case "select":
		return models.ActionSelect, true
	case "navigate":
		return models.ActionNavigate, true
	default:
		return "", false
	}
}
