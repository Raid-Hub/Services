package bungie

import (
	"fmt"
	"raidhub/lib/utils/logging"
	"sync"
)

// Bungie API logging constants
const (
	API_ERROR = "API_ERROR"
)

var logger = logging.NewLogger("BUNGIE_CLIENT")

func (c *BungieClient) GetActivityHistoryInChannel(membershipType int, membershipId int64, characterId int64, concurrentPages int, out chan int64) error {
	ch := make(chan int)

	// Fetch first page
	result, err := c.GetActivityHistoryPage(membershipType, membershipId, characterId, 250, 0, 4)
	if err != nil {
		return err
	}
	if !result.Success || result.Data == nil {
		return fmt.Errorf("failed to fetch first page of activity history")
	}

	for _, activity := range result.Data.Activities {
		out <- activity.ActivityDetails.InstanceId
	}

	open := true
	go func() {
		i := 1
		for open {
			ch <- i
			i++
		}
	}()

	var wg sync.WaitGroup
	for j := 0; j < concurrentPages; j++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for page := range ch {
				result, err := c.GetActivityHistoryPage(membershipType, membershipId, characterId, 250, page, 4)
				if err != nil {
					logger.Warn(API_ERROR, map[string]any{
						logging.OPERATION: "fetch_activity_history",
						logging.ERROR:     err.Error(),
					})
				}

				if !result.Success || result.Data == nil || len(result.Data.Activities) == 0 {
					break
				}

				for _, activity := range result.Data.Activities {
					out <- activity.ActivityDetails.InstanceId
				}
			}
		}()
	}

	wg.Wait()
	open = false
	return nil
}
