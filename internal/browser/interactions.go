package browser

// Real-browser interaction helpers. These replace chromedp calls and
// synthetic JS events that misbehave on real websites:
//   - clicks that silently land on an element covering the target,
//   - chromedp.Clear failing on empty <textarea>s,
//   - JS-dispatched hover events that never trigger CSS :hover,
//   - MouseEvent-based "drag" that HTML5 drag-and-drop pages ignore,
//   - NavigateBack/Forward waiting forever on back/forward-cache restores,
//   - chromedp.Poll aborting when the page navigates mid-poll.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

const interactionPollInterval = 50 * time.Millisecond

// automationFlags are Chrome switches every automation browser gets.
// Back/forward cache restores leave chromedp's DOM tracking pointing at the
// old document, so selectors never match after navigate_back/forward
// (Playwright disables the cache for the same reason).
var automationFlags = []chromedp.ExecAllocatorOption{
	chromedp.Flag("disable-back-forward-cache", true),
}

// withAutomationFlags returns a copy of opts with automationFlags appended;
// opts itself is never modified.
func withAutomationFlags(opts []chromedp.ExecAllocatorOption) []chromedp.ExecAllocatorOption {
	out := make([]chromedp.ExecAllocatorOption, 0, len(opts)+len(automationFlags))
	out = append(out, opts...)
	return append(out, automationFlags...)
}

// jsString encodes s as a JavaScript string literal.
func jsString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// isNavigationContextError reports whether err means the page's JS context
// went away because of a navigation, so the call can be retried on the new
// document.
func isNavigationContextError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	for _, s := range []string{
		"Cannot find context with specified id",
		"Execution context was destroyed",
		"Inspected target navigated or closed",
		"uniqueContextId not found",
	} {
		if strings.Contains(msg, s) {
			return true
		}
	}
	return false
}

// sleepCtx waits for d or until ctx is done.
func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// clickableScript returns "" when a click at the element's centre would hit
// the element (scrolling it to the viewport centre first if needed),
// "missing" if it is not in the DOM, or a description of whatever covers it.
const clickableScript = `(function(sel) {
	var el = document.querySelector(sel);
	if (!el) return 'missing';
	function hitAtCentre() {
		var r = el.getBoundingClientRect();
		if (r.width === 0 || r.height === 0) return null;
		return document.elementFromPoint(r.left + r.width / 2, r.top + r.height / 2);
	}
	function ok(at) {
		if (!at) return false;
		if (at === el || el.contains(at)) return true;
		if (el.labels) for (var i = 0; i < el.labels.length; i++) {
			if (el.labels[i] === at || el.labels[i].contains(at)) return true;
		}
		var root = el.getRootNode && el.getRootNode();
		return !!(root && root.host && at === root.host);
	}
	if (ok(hitAtCentre())) return '';
	el.scrollIntoView({block: 'center', inline: 'center'});
	var at = hitAtCentre();
	if (ok(at)) return '';
	if (!at) return 'nothing (element is outside the viewport or has no size)';
	var d = at.tagName.toLowerCase();
	if (at.id) d += '#' + at.id;
	if (typeof at.className === 'string' && at.className.trim()) d += '.' + at.className.trim().split(/\s+/).join('.');
	return d;
})(%s)`

// waitClickable waits until a real click at the element's centre would land
// on the element itself. It retries (covers can be transient, e.g. fading
// toasts) until ctx ends, then fails with what is covering the target.
func waitClickable(selector string) chromedp.ActionFunc {
	return func(ctx context.Context) error {
		js := fmt.Sprintf(clickableScript, jsString(selector))
		last := ""
		for {
			var res string
			err := chromedp.Evaluate(js, &res).Do(ctx)
			switch {
			case err == nil && res == "":
				return nil
			case err == nil:
				last = res
			case ctx.Err() == nil && !isNavigationContextError(err):
				return fmt.Errorf("check element %q is clickable: %w", selector, err)
			}
			if sleepCtx(ctx, interactionPollInterval) != nil {
				switch last {
				case "":
					return ctx.Err()
				case "missing":
					return fmt.Errorf("element %q disappeared before it could be clicked", selector)
				}
				return fmt.Errorf("element %q is covered by <%s>; refusing to click the wrong element", selector, last)
			}
		}
	}
}

// clearFieldScript empties an input/textarea (via the native value setter,
// so framework-controlled inputs see it) or a contenteditable element.
const clearFieldScript = `(function(sel) {
	var el = document.querySelector(sel);
	if (!el) return false;
	el.focus();
	if (!('value' in el) && el.isContentEditable) {
		if (el.textContent !== '') { el.textContent = ''; el.dispatchEvent(new Event('input', {bubbles: true})); }
		return true;
	}
	if (el.value === '') return true;
	var proto = el instanceof HTMLTextAreaElement ? HTMLTextAreaElement.prototype :
		el instanceof HTMLInputElement ? HTMLInputElement.prototype : null;
	var desc = proto && Object.getOwnPropertyDescriptor(proto, 'value');
	if (desc && desc.set) desc.set.call(el, ''); else el.value = '';
	el.dispatchEvent(new Event('input', {bubbles: true}));
	return true;
})(%s)`

func clearField(selector string) chromedp.ActionFunc {
	return func(ctx context.Context) error {
		var ok bool
		if err := chromedp.Evaluate(fmt.Sprintf(clearFieldScript, jsString(selector)), &ok).Do(ctx); err != nil {
			return fmt.Errorf("clear %q: %w", selector, err)
		}
		if !ok {
			return fmt.Errorf("clear %q: element not found", selector)
		}
		return nil
	}
}

type point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// centreOf scrolls the element to the viewport centre and returns its centre
// in viewport (CSS pixel) coordinates, as used by Input.dispatchMouseEvent.
func centreOf(ctx context.Context, selector string) (point, error) {
	js := fmt.Sprintf(`(function(sel) {
		var el = document.querySelector(sel);
		if (!el) return null;
		el.scrollIntoView({block: 'center', inline: 'center'});
		var r = el.getBoundingClientRect();
		return {x: r.left + r.width / 2, y: r.top + r.height / 2};
	})(%s)`, jsString(selector))
	var p *point
	if err := chromedp.Evaluate(js, &p).Do(ctx); err != nil {
		return point{}, err
	}
	if p == nil {
		return point{}, fmt.Errorf("element %q not found", selector)
	}
	return *p, nil
}

// mouseMoveTo moves the real (CDP) mouse to the element's centre, which
// fires mouseover/mouseenter/mousemove and applies CSS :hover.
func mouseMoveTo(selector string) chromedp.ActionFunc {
	return func(ctx context.Context) error {
		p, err := centreOf(ctx, selector)
		if err != nil {
			return err
		}
		return input.DispatchMouseEvent(input.MouseMoved, p.X, p.Y).Do(ctx)
	}
}

// html5DragScript performs an HTML5 drag-and-drop with a shared
// DataTransfer, following the spec: drop only fires if dragover was
// cancelled. Returns "dropped", "rejected", "cancelled" or "missing".
const html5DragScript = `(function(srcSel, dstSel) {
	var src = document.querySelector(srcSel), dst = document.querySelector(dstSel);
	if (!src || !dst) return 'missing';
	var dt = new DataTransfer();
	function centre(el) { var r = el.getBoundingClientRect(); return [r.left + r.width / 2, r.top + r.height / 2]; }
	function fire(el, type) {
		var c = centre(el);
		return el.dispatchEvent(new DragEvent(type, {bubbles: true, cancelable: true, composed: true,
			dataTransfer: dt, clientX: c[0], clientY: c[1]}));
	}
	if (!fire(src, 'dragstart')) return 'cancelled';
	fire(dst, 'dragenter');
	var accepted = !fire(dst, 'dragover');
	if (accepted) fire(dst, 'drop'); else fire(dst, 'dragleave');
	fire(src, 'dragend');
	return accepted ? 'dropped' : 'rejected';
})(%s, %s)`

// dragAndDrop drags source onto target. HTML5-draggable sources get a
// spec-shaped DragEvent sequence (CDP mouse input does not start native
// drags); anything else gets a real mouse press/move/release, which is what
// mouse-driven drag libraries listen for.
func dragAndDrop(source, target string) chromedp.ActionFunc {
	return func(ctx context.Context) error {
		var draggable bool
		if err := chromedp.Evaluate(fmt.Sprintf(`(function(s){var el=document.querySelector(s);return !!(el && el.draggable);})(%s)`,
			jsString(source)), &draggable).Do(ctx); err != nil {
			return err
		}
		if draggable {
			var outcome string
			if err := chromedp.Evaluate(fmt.Sprintf(html5DragScript, jsString(source), jsString(target)), &outcome).Do(ctx); err != nil {
				return err
			}
			switch outcome {
			case "dropped":
				return nil
			case "missing":
				return fmt.Errorf("source %q or target %q not found", source, target)
			case "rejected":
				return fmt.Errorf("target %q did not accept the drop (its dragover handler did not call preventDefault)", target)
			default:
				return fmt.Errorf("drag from %q was cancelled by the page", source)
			}
		}
		from, err := centreOf(ctx, source)
		if err != nil {
			return err
		}
		if err := input.DispatchMouseEvent(input.MouseMoved, from.X, from.Y).Do(ctx); err != nil {
			return err
		}
		if err := input.DispatchMouseEvent(input.MousePressed, from.X, from.Y).
			WithButton(input.Left).WithButtons(1).WithClickCount(1).Do(ctx); err != nil {
			return err
		}
		to, err := centreOf(ctx, target)
		if err != nil {
			return err
		}
		const steps = 10
		for i := 1; i <= steps; i++ {
			x := from.X + (to.X-from.X)*float64(i)/steps
			y := from.Y + (to.Y-from.Y)*float64(i)/steps
			if err := input.DispatchMouseEvent(input.MouseMoved, x, y).WithButton(input.Left).WithButtons(1).Do(ctx); err != nil {
				return err
			}
		}
		return input.DispatchMouseEvent(input.MouseReleased, to.X, to.Y).
			WithButton(input.Left).WithButtons(0).WithClickCount(1).Do(ctx)
	}
}

// navigateHistory moves delta entries through session history and waits for
// the document to be ready. Unlike chromedp.NavigateBack/Forward it does not
// wait for a load event, which never fires when Chrome restores the page from
// its back/forward cache.
func navigateHistory(delta int) chromedp.ActionFunc {
	name := "back"
	if delta > 0 {
		name = "forward"
	}
	return func(ctx context.Context) error {
		cur, entries, err := page.GetNavigationHistory().Do(ctx)
		if err != nil {
			return fmt.Errorf("navigate %s: read history: %w", name, err)
		}
		idx := int(cur) + delta
		if idx < 0 || idx >= len(entries) {
			return fmt.Errorf("navigate %s: no history entry", name)
		}
		if err := page.NavigateToHistoryEntry(entries[idx].ID).Do(ctx); err != nil {
			return fmt.Errorf("navigate %s: %w", name, err)
		}
		for {
			if now, _, err := page.GetNavigationHistory().Do(ctx); err == nil && int(now) == idx {
				var state string
				if err := chromedp.Evaluate(`document.readyState`, &state).Do(ctx); err == nil && state == "complete" {
					return nil
				}
			}
			if err := sleepCtx(ctx, interactionPollInterval); err != nil {
				return fmt.Errorf("navigate %s: page did not finish loading: %w", name, err)
			}
		}
	}
}

// pollTruthy evaluates expression (same form as chromedp.Poll: the body of
// `return (expression);`, promises awaited) until it is truthy or ctx ends.
// JS exceptions fail immediately as before, but a JS context destroyed by a
// navigation is retried on the new document.
func pollTruthy(expression string) chromedp.ActionFunc {
	js := fmt.Sprintf(`(async function() { return !!(await (function() { return (%s); })()); })()`, expression)
	return func(ctx context.Context) error {
		for {
			var ok bool
			err := chromedp.Evaluate(js, &ok, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
				return p.WithAwaitPromise(true)
			}).Do(ctx)
			switch {
			case err == nil && ok:
				return nil
			case err != nil && ctx.Err() == nil && !isNavigationContextError(err):
				return err
			}
			if err := sleepCtx(ctx, interactionPollInterval); err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					return fmt.Errorf("condition still false at timeout: %w", err)
				}
				return err
			}
		}
	}
}
