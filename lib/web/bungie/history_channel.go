package bungie

import (
	"fmt"
	"log"
	"sync"
)

func (c *BungieClient) GetActivityHistoryInChannel(membershipType int, membershipId int64, characterId int64, concurrentPages int, out chan int64) error {
	ch := make(chan int)

	// Fetch first page
	result, _, err := c.GetActivityHistoryPage(membershipType, membershipId, characterId, 250, 0, 4)
	if err != nil {
		return err
	}
	if result == nil || !result.Success || result.Data == nil {
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
				result, _, err := c.GetActivityHistoryPage(membershipType, membershipId, characterId, 250, page, 4)
				if err != nil {
					log.Printf("Error fetching activity history page: %s", err)
				}

				if result == nil || !result.Success || result.Data == nil || len(result.Data.Activities) == 0 {
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
