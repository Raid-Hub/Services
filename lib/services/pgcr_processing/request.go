package pgcr_processing

import (
	"context"
	"fmt"
	"net/http"
	netUrl "net/url"
	"raidhub/lib/monitoring/global_metrics"
	"raidhub/lib/utils/logging"
	"raidhub/lib/utils/network"
	"raidhub/lib/web/bungie"
	"time"
)

// fetchPGCR handles the HTTP request to Bungie API and processes error responses
// malformedRetryCount is optional - if > 0, adds "malformed_retry" query parameter
func FetchPGCR(ctx context.Context, instanceID int64, malformedRetryCount int) (PGCRResult, *bungie.DestinyPostGameCarnageReport) {
	var queryParams netUrl.Values
	if malformedRetryCount > 0 {
		queryParams = netUrl.Values{}
		queryParams.Set("malformed_retry", fmt.Sprintf("%d", malformedRetryCount))
	}
	start := time.Now()
	resp, err := bungie.PGCRClient.GetPGCR(ctx, instanceID, queryParams)
	duration := float64(time.Since(start).Milliseconds())

	// Record metric for all responses
	if resp.BungieErrorCode == bungie.Success {
		global_metrics.GetPostGameCarnageReportRequest.WithLabelValues("Success").Observe(duration)
	} else if resp.BungieErrorCode > 0 {
		global_metrics.GetPostGameCarnageReportRequest.WithLabelValues(resp.BungieErrorStatus).Observe(duration)
	}

	fields := map[string]any{
		logging.INSTANCE_ID:       instanceID,
		logging.BUNGIE_ERROR_CODE: resp.BungieErrorCode,
		logging.STATUS_CODE:       resp.HttpStatusCode,
	}
	if err != nil {
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

		// Transient errors, worth warning about
		case bungie.DestinyThrottledByGameServer:
			logger.Warn("PGCR_THROTTLED_BY_GAME_SERVER", err, fields)
			return ExternalError, nil
		case bungie.UnhandledException:
			logger.Warn("PGCR_UNHANDLED_EXCEPTION", err, fields)
			return ExternalError, nil
		}

		// Handle HTTP status codes
		switch resp.HttpStatusCode {
		case http.StatusForbidden:
			logger.Warn("PGCR_FORBIDDEN_ERROR", err, fields)
			return RateLimited, nil
		case http.StatusBadGateway:
			logger.Warn("PGCR_BAD_GATEWAY", err, fields)
			return ExternalError, nil
		case http.StatusServiceUnavailable:
			logger.Warn("PGCR_SERVICE_UNAVAILABLE", err, fields)
			return RateLimited, nil
		}

		logUnexpectedError(fields, err)
		return ExternalError, nil
	}

	// Check if Data is nil (can happen if API returns Response: null)
	if resp.Data == nil {
		logger.Warn("PGCR_DATA_NIL", nil, fields)
		return BadFormat, nil
	}

	return Success, resp.Data
}

func logUnexpectedError(
	fields map[string]any,
	err error,
) {
	netErr := network.CategorizeNetworkError(err)

	if netErr == nil {
		logger.Error("UNEXPECTED_PGCR_REQUEST_ERROR", err, fields)
		return
	}

	switch netErr.Type {
	case network.ErrorTypeTimeout:
		logger.Warn("PGCR_FETCH_TIMEOUT", err, fields)
	case network.ErrorTypeConnection:
		logger.Warn("PGCR_NETWORK_ERROR", err, fields)
	case network.ErrorTypeUnknown:
		logger.Error("UNEXPECTED_PGCR_REQUEST_ERROR", err, fields)
	}
}
