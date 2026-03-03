package bungie

import (
	"context"
	"sync"
	"time"

	"raidhub/lib/utils/logging"
)

const (
	// cloudflareThrottleDuration is how long all workers pause after a Cloudflare block is detected.
	// Each subsequent detection resets the timer, so a sustained block extends the pause.
	cloudflareThrottleDuration = 60 * time.Second
)

var (
	cloudflareThrottleMu  sync.Mutex
	cloudflareThrottleCh  chan struct{} // closed when NOT throttled; open (blocks) when throttled
	cloudflareIsThrottled bool
	cloudflareGeneration  int // incremented on each Signal to invalidate stale timer callbacks
	cloudflareThrottleLog = logging.NewLogger("CLOUDFLARE_THROTTLE")
)

func init() {
	// Start in unthrottled state: a closed channel unblocks all Select receivers immediately.
	cloudflareThrottleCh = make(chan struct{})
	close(cloudflareThrottleCh)
}

// SignalCloudflareThrottle activates a global pause when a Cloudflare block is detected.
// All callers of WaitForCloudflareThrottle will block for cloudflareThrottleDuration.
// If already throttled, the cooldown period is extended from now.
func SignalCloudflareThrottle() {
	cloudflareThrottleMu.Lock()
	defer cloudflareThrottleMu.Unlock()

	cloudflareGeneration++
	gen := cloudflareGeneration

	if !cloudflareIsThrottled {
		// Replace the closed channel with a new open channel so waiters will block.
		cloudflareIsThrottled = true
		cloudflareThrottleCh = make(chan struct{})
		cloudflareThrottleLog.Warn("CLOUDFLARE_THROTTLE_ACTIVATED", nil, map[string]any{
			"duration_s": int(cloudflareThrottleDuration.Seconds()),
		})
	}

	// Capture the current channel so the timer callback releases the right set of waiters.
	// Using a generation counter ensures only the latest timer callback fires; earlier callbacks
	// (from previous or superseded signals) are no-ops because their generation no longer matches.
	ch := cloudflareThrottleCh
	time.AfterFunc(cloudflareThrottleDuration, func() {
		cloudflareThrottleMu.Lock()
		defer cloudflareThrottleMu.Unlock()
		if cloudflareIsThrottled && cloudflareGeneration == gen {
			cloudflareIsThrottled = false
			close(ch) // Release all current waiters atomically
			cloudflareThrottleLog.Info("CLOUDFLARE_THROTTLE_CLEARED", nil)
		}
	})
}

// WaitForCloudflareThrottle blocks until the global Cloudflare throttle is inactive or ctx is done.
// Returns ctx.Err() if the context is cancelled while waiting.
func WaitForCloudflareThrottle(ctx context.Context) error {
	cloudflareThrottleMu.Lock()
	ch := cloudflareThrottleCh
	cloudflareThrottleMu.Unlock()

	// If ch is closed (not throttled), this select arm returns immediately with no goroutine spawned.
	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
