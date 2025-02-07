package pgcr

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"raidhub/packages/bungie"
	"raidhub/packages/monitoring"
	"raidhub/packages/pgcr_types"
	"sync"
	"time"
)

type PGCRResult int

const (
	Success                PGCRResult = 1
	NonRaid                PGCRResult = 2
	NotFound               PGCRResult = 3
	SystemDisabled         PGCRResult = 4
	InsufficientPrivileges PGCRResult = 5
	BadFormat              PGCRResult = 6
	InternalError          PGCRResult = 7
	DecodingError          PGCRResult = 8
)

var (
	pgcrUrlBase string
	once        sync.Once
)

func getPgcrURL() string {
	once.Do(func() {
		pgcrUrlBase = os.Getenv("PGCR_URL_BASE")
		if pgcrUrlBase == "" {
			pgcrUrlBase = "https://stats.bungie.net/"
		}
	})
	return pgcrUrlBase
}

func FetchAndProcessPGCR(client *http.Client, instanceID int64, apiKey string) (PGCRResult, *pgcr_types.ProcessedActivity, *bungie.DestinyPostGameCarnageReport, error) {
	start := time.Now()
	decoder, statusCode, cleanup, err := bungie.GetPGCR(client, getPgcrURL(), instanceID, apiKey)
	if err != nil {
		log.Printf("Error fetching instanceId %d: %s", instanceID, err)
		return InternalError, nil, nil, err
	}
	defer cleanup()

	if statusCode != http.StatusOK {
		var data bungie.BungieError
		if err := decoder.Decode(&data); err != nil {
			log.Printf("Error decoding %d response for instanceId %d: %s", statusCode, instanceID, err)
			monitoring.GetPostGameCarnageReportRequest.WithLabelValues(fmt.Sprintf("Unknown%d", statusCode)).Observe(float64(time.Since(start).Milliseconds()))
			// Handle a few cases here
			if statusCode == 404 {
				return NotFound, nil, nil, err
			} else if statusCode == 429 {
				return SystemDisabled, nil, nil, err
			}
			
			if statusCode == 403 {
				// Rate Limit
				time.Sleep(120 * time.Second)
			}
			return DecodingError, nil, nil, err
		}
		monitoring.GetPostGameCarnageReportRequest.WithLabelValues(data.ErrorStatus).Observe(float64(time.Since(start).Milliseconds()))

		defer func() {
			if data.ThrottleSeconds > 0 {
				log.Printf("Throttled: %d seconds", data.ThrottleSeconds)
				time.Sleep(time.Duration(data.ThrottleSeconds) * time.Second)
			}
		}()

		if data.ErrorCode == 1653 {
			// PGCRNotFound
			return NotFound, nil, nil, fmt.Errorf("%s", data.ErrorStatus)
		}

		log.Printf("Error response for instanceId %d: %s (%d)", instanceID, data.Message, data.ErrorCode)
		if data.ErrorCode == 5 {
			// SystemDisabled
			return SystemDisabled, nil, nil, fmt.Errorf("%s", data.ErrorStatus)
		} else if data.ErrorCode == 1672 {
			// BabelTimeout
			return NotFound, nil, nil, fmt.Errorf("%s", data.ErrorStatus)
		} else if data.ErrorCode == 12 {
			// InsufficientPrivileges, redacted
			return InsufficientPrivileges, nil, nil, fmt.Errorf("%s", data.ErrorStatus)
		}

		return DecodingError, nil, nil, nil
	}

	var data bungie.DestinyPostGameCarnageReportResponse
	if err := decoder.Decode(&data); err != nil {
		log.Printf("Error decoding response for instanceId %d: %s", instanceID, err)
		return DecodingError, nil, nil, err
	}
	monitoring.GetPostGameCarnageReportRequest.WithLabelValues(data.ErrorStatus).Observe(float64(time.Since(start).Milliseconds()))

	if data.Response.ActivityDetails.Mode != 4 {
		return NonRaid, nil, &data.Response, nil
	}

	pgcr, err := ProcessDestinyReport(&data.Response)
	if err != nil {
		log.Println(err)
		return BadFormat, nil, nil, err
	}

	return Success, pgcr, &data.Response, nil
}
