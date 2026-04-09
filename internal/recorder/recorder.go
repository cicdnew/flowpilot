package recorder

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"flowpilot/internal/models"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"

	"flowpilot/internal/logs"
)

// EventHandler is called when a new step is recorded during a browser session.
type EventHandler func(step models.RecordedStep)

// Recorder opens a headless=false Chrome session and captures user interactions via CDP.
type Recorder struct {
	mu            sync.Mutex
	parentCtx     context.Context    //nolint:godre:S8242 -- browser session lifetime context passed from caller
	handler       EventHandler
	flowID        string
	stepIndex     int
	allocCtx      context.Context    //nolint:godre:S8242 -- chromedp allocator lifetime context
	allocCancel   context.CancelFunc
	browserCtx    context.Context    //nolint:godre:S8242 -- chromedp browser lifetime context
	browserCancel context.CancelFunc
	netLogger     *logs.NetworkLogger
	wsLogger      *logs.WebSocketLogger
	snapshotter   *Snapshotter
	activeTabID   target.ID
	cdp           CDPClient
}

func New(parentCtx context.Context, flowID string, handler EventHandler) *Recorder {
	return &Recorder{parentCtx: parentCtx, handler: handler, flowID: flowID, stepIndex: 0, cdp: chromeCDPClient{}}
}

func (r *Recorder) Start(url string) error {
	opts := make([]chromedp.ExecAllocatorOption, len(chromedp.DefaultExecAllocatorOptions))
	copy(opts, chromedp.DefaultExecAllocatorOptions[:])

	opts = append(opts,
		chromedp.Flag("headless", false),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("enable-unsafe-swiftshader", true),
	)

	r.allocCtx, r.allocCancel = chromedp.NewExecAllocator(r.parentCtx, opts...)
	r.browserCtx, r.browserCancel = chromedp.NewContext(r.allocCtx)
	r.netLogger = logs.NewNetworkLogger(r.flowID)
	r.wsLogger = logs.NewWebSocketLogger(r.flowID)

	r.registerListeners()

	if err := r.enableDomains(); err != nil {
		return err
	}

	if err := r.installCaptureScript(); err != nil {
		return err
	}

	if err := r.cdp.Run(r.browserCtx, chromedp.Navigate(url)); err != nil {
		return fmt.Errorf("navigate to %s: %w", url, err)
	}

	if err := r.injectCaptureScript(); err != nil {
		return fmt.Errorf("inject capture script: %w", err)
	}

	return nil
}

func (r *Recorder) registerListeners() {
	r.cdp.ListenTarget(r.browserCtx, func(ev any) {
		r.handleEvent(ev)
	})
}

func (r *Recorder) handleEvent(ev any) {
	switch e := ev.(type) {
	case *runtime.EventBindingCalled:
		if e.Name == bindingName {
			r.handleBindingCall(e.Payload)
		}
	case *page.EventFrameNavigated:
		r.handleFrameNavigated(e)
	case *target.EventTargetInfoChanged:
		r.handleTargetInfoChanged(e)
	case *network.EventRequestWillBeSent:
		r.handleNetworkRequest(e)
	case *network.EventResponseReceived:
		if r.netLogger != nil {
			r.netLogger.HandleResponseReceived(e)
		}
	case *network.EventLoadingFinished:
		if r.netLogger != nil {
			r.netLogger.HandleLoadingFinished(e, nil)
		}
	case *network.EventLoadingFailed:
		if r.netLogger != nil {
			r.netLogger.HandleLoadingFailed(e.RequestID)
		}
	case *network.EventWebSocketCreated:
		r.handleWebSocketCreated(e)
	case *network.EventWebSocketHandshakeResponseReceived:
		if r.wsLogger != nil {
			r.wsLogger.HandleHandshake(e)
		}
	case *network.EventWebSocketFrameSent:
		if r.wsLogger != nil {
			r.wsLogger.HandleFrameSent(e)
		}
	case *network.EventWebSocketFrameReceived:
		if r.wsLogger != nil {
			r.wsLogger.HandleFrameReceived(e)
		}
	case *network.EventWebSocketClosed:
		if r.wsLogger != nil {
			r.wsLogger.HandleClosed(e)
		}
	case *network.EventWebSocketFrameError:
		if r.wsLogger != nil {
			r.wsLogger.HandleFrameError(e)
		}
	}
}

func (r *Recorder) handleFrameNavigated(e *page.EventFrameNavigated) {
	if e.Frame == nil || e.Frame.ParentID != "" {
		return
	}
	r.RecordStep(models.ActionNavigate, "", e.Frame.URL)
}

func (r *Recorder) handleTargetInfoChanged(e *target.EventTargetInfoChanged) {
	if e.TargetInfo == nil || e.TargetInfo.Type != "page" {
		return
	}
	r.mu.Lock()
	if r.activeTabID != "" && e.TargetInfo.TargetID != r.activeTabID {
		r.activeTabID = e.TargetInfo.TargetID
		r.mu.Unlock()
		r.RecordStep(models.ActionTabSwitch, "", e.TargetInfo.URL)
	} else {
		if r.activeTabID == "" {
			r.activeTabID = e.TargetInfo.TargetID
		}
		r.mu.Unlock()
	}
}

func (r *Recorder) handleNetworkRequest(e *network.EventRequestWillBeSent) {
	if r.netLogger == nil {
		return
	}
	r.mu.Lock()
	idx := r.stepIndex
	r.mu.Unlock()
	r.netLogger.SetStepIndex(idx)
	r.netLogger.HandleRequestWillBeSent(e)
}

func (r *Recorder) handleWebSocketCreated(e *network.EventWebSocketCreated) {
	if r.wsLogger == nil {
		return
	}
	r.mu.Lock()
	idx := r.stepIndex
	r.mu.Unlock()
	r.wsLogger.SetStepIndex(idx)
	r.wsLogger.HandleCreated(e)
}

func (r *Recorder) enableDomains() error {
	if err := r.cdp.Run(r.browserCtx, runtime.Enable()); err != nil {
		return fmt.Errorf("enable runtime domain: %w", err)
	}
	if err := r.cdp.Run(r.browserCtx, page.Enable()); err != nil {
		return fmt.Errorf("enable page domain: %w", err)
	}
	if err := r.cdp.Run(r.browserCtx, network.Enable()); err != nil {
		return fmt.Errorf("enable network domain: %w", err)
	}
	if err := r.cdp.Run(r.browserCtx, runtime.AddBinding(bindingName)); err != nil {
		return fmt.Errorf("add binding %s: %w", bindingName, err)
	}
	return nil
}

func (r *Recorder) installCaptureScript() error {
	err := r.cdp.Run(r.browserCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		_, err := page.AddScriptToEvaluateOnNewDocument(captureScript).Do(ctx)
		return err
	}))
	if err != nil {
		return fmt.Errorf("install capture script: %w", err)
	}
	return nil
}

func (r *Recorder) injectCaptureScript() error {
	return r.cdp.Run(r.browserCtx, chromedp.Evaluate(captureScript, nil))
}

func (r *Recorder) handleBindingCall(payload string) {
	action, selector, value, err := parseBindingPayload(payload)
	if err != nil {
		return
	}
	r.RecordStep(action, selector, value)
}

func (r *Recorder) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.browserCtx != nil {
		_ = chromedp.Cancel(r.browserCtx)
	}
	if r.browserCancel != nil {
		r.browserCancel()
		r.browserCancel = nil
	}
	if r.allocCancel != nil {
		r.allocCancel()
		r.allocCancel = nil
	}
	r.browserCtx = nil
	r.allocCtx = nil
}

func (r *Recorder) BrowserCtx() context.Context {
	return r.browserCtx
}

func (r *Recorder) FlowID() string {
	return r.flowID
}

func (r *Recorder) NetworkLogs() []models.NetworkLog {
	if r.netLogger == nil {
		return nil
	}
	return r.netLogger.Logs()
}

func (r *Recorder) WebSocketLogs() []models.WebSocketLog {
	if r.wsLogger == nil {
		return nil
	}
	return r.wsLogger.Logs()
}

func (r *Recorder) SetWSCallback(cb logs.WSEventCallback) {
	if r.wsLogger != nil {
		r.wsLogger.SetCallback(cb)
	}
}

func (r *Recorder) SetSnapshotter(s *Snapshotter) {
	r.snapshotter = s
}

func (r *Recorder) RecordStep(action models.StepAction, selector, value string) {
	if r.handler == nil {
		return
	}
	r.mu.Lock()
	idx := r.stepIndex
	r.stepIndex++
	snapshotter := r.snapshotter
	browserCtx := r.browserCtx
	r.mu.Unlock()

	step := models.RecordedStep{
		Index:     idx,
		Action:    action,
		Selector:  selector,
		Value:     value,
		Timestamp: time.Now(),
	}
	if snapshotter != nil && browserCtx != nil {
		if snap, err := snapshotter.CaptureSnapshot(browserCtx, r.flowID, idx); err == nil {
			step.SnapshotID = snap.ID
		} else {
			log.Printf("recorder: snapshot step %d: %v", idx, err)
		}
	}
	r.handler(step)
}
