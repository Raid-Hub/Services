package bungie

import (
	"context"
	"fmt"
	"raidhub/lib/utils/logging"
	"sync"
)

// Bungie API logging constants
const (
	API_ERROR = "API_ERROR"
)

var logger = logging.NewLogger("BUNGIE_CLIENT")

// ActivityHistoryResult contains the result of fetching activity history
type ActivityHistoryResult struct {
	Error            error
	PrivacyErrorCode int // Non-zero if privacy restriction detected
}

// GetActivityHistoryInChannel fetches activity history pages and sends instance IDs to the output channel.
// Returns an ActivityHistoryResult indicating if a privacy error occurred.
func (c *BungieClient) GetActivityHistoryInChannel(ctx context.Context, membershipType int, membershipId int64, characterId int64, concurrentPages int, out chan int64) ActivityHistoryResult {
	// Fetch first page to check for privacy errors
	result, err := c.GetActivityHistoryPage(ctx, membershipType, membershipId, characterId, 250, 0, 4)
	if result.BungieErrorCode == DestinyPrivacyRestriction {
		return ActivityHistoryResult{PrivacyErrorCode: result.BungieErrorCode}
	} else if err != nil {
		return ActivityHistoryResult{Error: err}
	} else if result.Data == nil {
		return ActivityHistoryResult{Error: fmt.Errorf("failed to fetch first page of activity history: %s [%d]", result.BungieErrorStatus, result.BungieErrorCode)}
	}

	// Send first page activities
	for _, activity := range result.Data.Activities {
		out <- activity.ActivityDetails.InstanceId
	}

	// Fetch remaining pages concurrently
	pageChan := make(chan int, concurrentPages)
	done := make(chan bool, 1)
	var wg sync.WaitGroup

	// Start worker goroutines
	for i := 0; i < concurrentPages; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case page := <-pageChan:
					result, err := c.GetActivityHistoryPage(ctx, membershipType, membershipId, characterId, 250, page, 4)
					if err != nil {
						logger.Warn(API_ERROR, err, map[string]any{
							logging.OPERATION: "fetch_activity_history",
						})
						continue
					}

					// Stop if no success, no data, or no activities
					if result.Data == nil || len(result.Data.Activities) == 0 {
						select {
						case done <- true:
						default:
						}
						return
					}

					// Send activities to output channel
					for _, activity := range result.Data.Activities {
						out <- activity.ActivityDetails.InstanceId
					}
				case <-done:
					return
				}
			}
		}()
	}

	// Send page numbers to workers, starting from page 1
	go func() {
		page := 1
		for {
			select {
			case pageChan <- page:
				page++
			case <-done:
				close(pageChan)
				return
			}
		}
	}()

	wg.Wait()
	close(done)
	return ActivityHistoryResult{}
}
