package pgcr_blocked

import (
	"log"
	"math/rand"
	"net/http"
	"raidhub/packages/async"
	"raidhub/packages/monitoring"
	"raidhub/packages/pgcr"
	"strconv"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	retryDelayTime = 10000
)

var (
	pgcrSuccess sync.Map
)

func process_request(qw *async.QueueWorker, msg amqp.Delivery, client *http.Client, apiKey string) {
	defer func() {
		if err := msg.Ack(false); err != nil {
			log.Printf("Failed to acknowledge message: %v", err)
		}
	}()

	instanceId, err := strconv.ParseInt(string(msg.Body), 10, 64)
	if err != nil {
		log.Fatalf("Failed to parse message body: %s", err)
		return
	}

	randomVariation := retryDelayTime / 3

	i := 0
	errCount := 0
	for {
		qw.Wg.Wait()

		result, activity, raw, err := pgcr.FetchAndProcessPGCR(client, instanceId, apiKey)
		if err != nil && result != pgcr.NotFound {
			log.Println(err)
		}

		// Handle the result
		if result == pgcr.NonRaid {
			monitoring.FloodgatesRecent.Set(float64(instanceId))
			break
		} else if result == pgcr.Success {
			_, _, err := pgcr.StorePGCR(activity, raw, qw.Db, outgoing)
			if err != nil {
				errCount++
				pgcr.WriteMissedLog(instanceId)
			} else {
				// Successfully processed the PGCR
				monitoring.FloodgatesRecent.Set(float64(instanceId))
				break
			}
		} else if result == pgcr.InsufficientPrivileges {
			if !isUnblocked() {
				time.Sleep(time.Duration(60) * time.Second)
				continue
			} else {
				errCount++
				log.Printf("PGCR %d is blocked (%d), but unblocked PGCRs are available.", instanceId, errCount)
			}
		} else if result == pgcr.NotFound {
			pgcr.WriteMissedLog(instanceId)
			break
		} else if result == pgcr.SystemDisabled {
			time.Sleep(45 * time.Second)
			continue
		} else {
			errCount++
			time.Sleep(time.Duration(5*errCount*errCount) * time.Second)
			pgcr.WriteMissedLog(instanceId)
		}

		// If we have not found the instance id after some time
		if errCount > 3 {
			log.Printf("PGCR %d is still blocked while unblocked PGCRs are available. Marking as missed.", instanceId)
			pgcr.WriteMissedLog(instanceId)
			return
		}

		timeout := time.Duration((retryDelayTime - randomVariation + rand.Intn(retryDelayTime*(i+1)))) * time.Millisecond
		i++
		time.Sleep(timeout)
	}

	go tempStoreId(instanceId)
}

func tempStoreId(instanceId int64) {
	pgcrSuccess.Store(instanceId, time.Now())
	// Wait for 3 minutes before removing the ID from the cache
	time.Sleep(3 * time.Minute)
	pgcrSuccess.Delete(instanceId)
}

func isUnblocked() bool {
	countUnblocked := 0

	// See how many PGCRs are unblocked
	pgcrSuccess.Range(func(_, _ any) bool {
		countUnblocked++

		// exit early if we have enough
		return countUnblocked <= 3
	})

	// If most PGCRs have unblocked, consider skipping it
	return countUnblocked > 3
}
