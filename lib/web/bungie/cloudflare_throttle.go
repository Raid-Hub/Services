package bungie

import (
	"context"
	"sync"
	"time"

	"raidhub/lib/utils/logging"
)

const (
	// cloudflareThrottleDuration is how long all workers pause after the throttle is activated.
	cloudflareThrottleDuration = 60 * time.Second

	// cloudflareThrottleWindow is the sliding window over which errors are counted.
	cloudflareThrottleWindow = 10 * time.Second

	// cloudflareThrottleMinErrors is the minimum number of Cloudflare errors within
	// cloudflareThrottleWindow required to activate the global throttle.
	cloudflareThrottleMinErrors = 3
)

var (
	cloudflareThrottleMu  sync.Mutex
	cloudflareThrottleCh  chan struct{} // closed when NOT throttled; open (blocks) when throttled
	cloudflareIsThrottled bool
	cloudflareGeneration  int         // incremented on each new throttle activation; guards against stale timer callbacks
	cloudflareErrorTimes  []time.Time // sliding window of recent Cloudflare error timestamps
	cloudflareThrottleLog = logging.NewLogger("CLOUDFLARE_THROTTLE")
)

func init() {
	// Start in unthrottled state: a closed channel unblocks all Select receivers immediately.
	cloudflareThrottleCh = make(chan struct{})
	close(cloudflareThrottleCh)
}

// SignalCloudflareThrottle records a Cloudflare error and activates a global pause when
// cloudflareThrottleMinErrors errors occur within cloudflareThrottleWindow.
// All callers of WaitForCloudflareThrottle will block for cloudflareThrottleDuration.
// If the throttle is already active, this is a no-op.
func SignalCloudflareThrottle() {
	cloudflareThrottleMu.Lock()
	defer cloudflareThrottleMu.Unlock()

	now := time.Now()

	// Append the current error and prune events outside the sliding window.
	cloudflareErrorTimes = append(cloudflareErrorTimes, now)
	cutoff := now.Add(-cloudflareThrottleWindow)
	start := 0
	for start < len(cloudflareErrorTimes) && cloudflareErrorTimes[start].Before(cutoff) {
		start++
	}
	cloudflareErrorTimes = cloudflareErrorTimes[start:]

	// Only activate the throttle once the threshold is reached.
	if len(cloudflareErrorTimes) < cloudflareThrottleMinErrors {
		return
	}

	// If already throttled, nothing to do — the existing timer will clear it.
	if cloudflareIsThrottled {
		return
	}

	cloudflareGeneration++
	gen := cloudflareGeneration

	// Replace the closed channel with a new open channel so waiters will block.
	cloudflareIsThrottled = true
	cloudflareThrottleCh = make(chan struct{})
	cloudflareThrottleLog.Warn("CLOUDFLARE_THROTTLE_ACTIVATED", nil, map[string]any{
		"duration_s":  int(cloudflareThrottleDuration.Seconds()),
		"error_count": len(cloudflareErrorTimes),
		"window_s":    int(cloudflareThrottleWindow.Seconds()),
	})

	// Capture the current channel so the timer callback releases the right set of waiters.
	// The generation counter guards against the unlikely race where this timer fires after
	// the throttle has already been cleared and a new throttle period has started.
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
