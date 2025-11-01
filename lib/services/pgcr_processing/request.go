package pgcr_processing

import (
	"net/http"
	"raidhub/lib/monitoring/global_metrics"
	"raidhub/lib/utils/logging"
	"raidhub/lib/web/bungie"
	"strings"
	"time"
)

// fetchPGCR handles the HTTP request to Bungie API and processes error responses
func FetchPGCR(instanceID int64) (PGCRResult, *bungie.DestinyPostGameCarnageReport) {
	start := time.Now()
	resp, err := bungie.PGCRClient.GetPGCR(instanceID)
	if resp.BungieErrorCode > 0 {
		global_metrics.GetPostGameCarnageReportRequest.WithLabelValues(resp.BungieErrorStatus).Observe(float64(time.Since(start).Milliseconds()))
	}

	if !resp.Success {
		fields := map[string]any{
			logging.INSTANCE_ID:       instanceID,
			logging.BUNGIE_ERROR_CODE: resp.BungieErrorCode,
			logging.ERROR:             err.Error(),
			logging.STATUS_CODE:       resp.HttpStatusCode,
		}

		// Handle Bungie error codes
		switch resp.BungieErrorCode {
		case bungie.PGCRNotFound:
			logger.Debug("PGCR_NOT_FOUND", fields)
			return NotFound, nil
		case bungie.SystemDisabled:
			logger.Info("BUNGIE_SYSTEM_DISABLED", fields)
			// Signal immediate check for Destiny2 system availability
			bungie.SignalSystemDisabled("Destiny2")
			return SystemDisabled, nil
		case bungie.InsufficientPrivileges:
			logger.Info("PGCR_INSUFFICIENT_PRIVILEGES", fields)
			return InsufficientPrivileges, nil
		case bungie.DestinyThrottledByGameServer:
			logger.Warn("PGCR_THROTTLED_BY_GAME_SERVER", fields)
			return ExternalError, nil
		}

		// Handle HTTP status codes
		switch resp.HttpStatusCode {
		case http.StatusForbidden:
			logger.Warn("PGCR_FORBIDDEN_ERROR", fields)
			return RateLimited, nil
		case http.StatusServiceUnavailable:
			logger.Warn("PGCR_SERVICE_UNAVAILABLE", fields)
			return RateLimited, nil
		}

		logUnexpectedError(fields, err)
		return ExternalError, nil
	}

	return Success, resp.Data
}

func logUnexpectedError(
	fields map[string]any,
	err error,
) {
	errStr := err.Error()

	isTimeout := strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded")
	if isTimeout {
		logger.Warn("PGCR_FETCH_TIMEOUT", fields)
		return
	}

	isConnectionError := strings.Contains(errStr, "connection refused") || strings.Contains(errStr, "connection reset")
	if isConnectionError {
		logger.Warn("PGCR_CONNECTION_ERROR", fields)
		return
	}

	logger.Error("UNEXPECTED_PGCR_REQUEST_ERROR", fields)
}
