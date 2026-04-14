package subscriptions

import (
	"context"
	"strings"
	"unicode/utf8"

	"raidhub/lib/database/postgres"
	"raidhub/lib/env"
	"raidhub/lib/messaging/processing"
)

const maxLastDeliveryErrorRunes = 512

// RecordDestinationDeliveryOutcome updates subscriptions.destination delivery health.
// Retryable errors increment the counter only on the final attempt (retryCount >= maxRetries) or when
// the error is unretryable (permanent), so transient retries do not inflate consecutive_failures.
func RecordDestinationDeliveryOutcome(ctx context.Context, destinationID int64, deliveryErr error, retryCount int, maxRetries int) {
	if destinationID <= 0 {
		return
	}
	if deliveryErr == nil {
		_ = resetDestinationDeliverySuccess(ctx, destinationID)
		return
	}
	if !shouldRecordFailedDelivery(deliveryErr, retryCount, maxRetries) {
		return
	}
	permanent := processing.IsUnretryableError(deliveryErr)
	_ = recordDestinationDeliveryFailure(ctx, destinationID, deliveryErr, permanent, env.SubscriptionDestDisableAfterConsecutiveFailures)
}

func shouldRecordFailedDelivery(err error, retryCount, maxRetries int) bool {
	if processing.IsUnretryableError(err) {
		return true
	}
	if maxRetries <= 0 {
		return true
	}
	return retryCount >= maxRetries
}

func resetDestinationDeliverySuccess(ctx context.Context, destinationID int64) error {
	_, err := postgres.DB.ExecContext(ctx, `
		UPDATE subscriptions.destination
		SET consecutive_delivery_failures = 0,
		    last_delivery_success_at = NOW(),
		    last_delivery_error = NULL,
		    updated_at = NOW()
		WHERE id = $1`, destinationID)
	return err
}

func recordDestinationDeliveryFailure(ctx context.Context, destinationID int64, deliveryErr error, permanent bool, disableAfter int) error {
	errText := truncateLastDeliveryError(deliveryErr.Error())
	_, err := postgres.DB.ExecContext(ctx, `
		UPDATE subscriptions.destination AS d
		SET consecutive_delivery_failures = d.consecutive_delivery_failures + 1,
		    last_delivery_failure_at = NOW(),
		    last_delivery_error = $2,
		    updated_at = NOW(),
		    is_active = CASE
		      WHEN NOT d.is_active THEN d.is_active
		      WHEN ($3 OR d.consecutive_delivery_failures + 1 >= $4) THEN false
		      ELSE d.is_active
		    END,
		    deactivated_at = CASE
		      WHEN d.is_active AND ($3 OR d.consecutive_delivery_failures + 1 >= $4) THEN NOW()
		      ELSE d.deactivated_at
		    END,
		    deactivation_reason = CASE
		      WHEN d.is_active AND ($3 OR d.consecutive_delivery_failures + 1 >= $4) THEN
		        CASE WHEN $3 THEN 'permanent_delivery_error' ELSE 'consecutive_failures' END
		      ELSE d.deactivation_reason
		    END
		WHERE d.id = $1`,
		destinationID, errText, permanent, disableAfter)
	return err
}

func truncateLastDeliveryError(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if utf8.RuneCountInString(s) <= maxLastDeliveryErrorRunes {
		return s
	}
	r := []rune(s)
	return string(r[:maxLastDeliveryErrorRunes-1]) + "…"
}
