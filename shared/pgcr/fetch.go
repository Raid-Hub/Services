package pgcr

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"raidhub/shared/bungie"
	"raidhub/shared/monitoring"
	"raidhub/shared/pgcr_types"
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
	ExternalError          PGCRResult = 9
	RateLimited            PGCRResult = 10
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

var now = time.Now()

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
			switch statusCode {
			case 404:
				return NotFound, nil, nil, err
			case 429:
				return SystemDisabled, nil, nil, err
			case 403:
				return RateLimited, nil, nil, err
			}
			return DecodingError, nil, nil, err
		}
		monitoring.GetPostGameCarnageReportRequest.WithLabelValues(data.ErrorStatus).Observe(float64(time.Since(start).Milliseconds()))

		bungieErr := fmt.Errorf("%s", data.ErrorStatus)
		if data.ErrorCode == 1653 {
			// PGCRNotFound
			return NotFound, nil, nil, bungieErr
		}

		log.Printf("Error response for instanceId %d: %s (%d) - %s", instanceID, data.ErrorStatus, data.ErrorCode, data.Message)

		if data.ErrorCode == 5 {
			return SystemDisabled, nil, nil, bungieErr
		} else if data.ErrorCode == 12 {
			return InsufficientPrivileges, nil, nil, bungieErr
		} else if statusCode == 403 {
			return RateLimited, nil, nil, bungieErr
		}

		return ExternalError, nil, nil, bungieErr
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
