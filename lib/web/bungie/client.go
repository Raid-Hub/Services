package bungie

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	netUrl "net/url"
	"regexp"
	"strings"
	"time"

	"raidhub/lib/utils/logging"
	"raidhub/lib/utils/network"
	"raidhub/lib/utils/retry"
)

var clientLogger = logging.NewLogger("BUNGIE_CLIENT")

type BungieHttpResult[T any] struct {
	Data              *T
	HttpStatusCode    int
	BungieErrorCode   int
	BungieErrorStatus string
	MessageData       map[string]any
}

type BungieClient struct {
	httpClient *http.Client
	scheme     string
	host       string
	apiKey     string
}

func makeNonStandardHttpResult[T any](statusCode int) BungieHttpResult[T] {
	return BungieHttpResult[T]{
		HttpStatusCode:    statusCode,
		BungieErrorCode:   OtherError,
		BungieErrorStatus: "",
		Data:              nil,
		MessageData:       nil,
	}
}

type BungieResponseParseError struct {
	StatusCode  int
	ContentType string
	Title       string
	Operation   string
}

func (e *BungieResponseParseError) Error() string {
	if e.Title == "" || e.ContentType == "" {
		return fmt.Sprintf("%s: unexpected bungie %d response", e.Operation, e.StatusCode)
	}
	return fmt.Sprintf("%s: unexpected bungie %d %s response: '%s'", e.Operation, e.StatusCode, e.ContentType, e.Title)
}

// isKnownHTMLErrorPage checks if the error is a known HTML error page that should not be sent to Sentry
// These are expected/recoverable conditions that are handled by retry logic
func (e *BungieResponseParseError) isKnownHTMLErrorPage() bool {
	if e.ContentType == "" || e.Title == "" {
		return false
	}

	// Check if it's an HTML response
	if !strings.Contains(strings.ToLower(e.ContentType), "text/html") {
		return false
	}

	titleLower := strings.ToLower(e.Title)

	// Known Cloudflare error pages
	knownPages := []string{
		"attention required! | cloudflare",
		"just a moment...",
		"checking your browser",
		"ddos protection by cloudflare",
		"access denied",
	}

	for _, knownPage := range knownPages {
		if strings.Contains(titleLower, knownPage) {
			return true
		}
	}

	return false
}

func get[T any](ctx context.Context, c *BungieClient, url netUrl.URL, operation string, params map[string]any) (BungieHttpResult[T], error) {
	// Wraps the get in 2 layers of retry:
	// Inner layer retries timeout and connection errors (excludes BungieError instances)
	// Outer layer retries Cloudflare errors
	return retry.WithRetryForResult(ctx, network.CloudflareRetryConfig(clientLogger, params), func(attempt int) (BungieHttpResult[T], error) {
		if attempt > 1 {
			// add a query parameter to the url to indicate the retry attempt
			queryValues := url.Query()
			queryValues.Add("retry", fmt.Sprintf("%d", attempt))
			url.RawQuery = queryValues.Encode()
		}
		return retry.WithRetryForResult(ctx, BungieRetryConfig(), func(_ int) (BungieHttpResult[T], error) {
			return getInternal[T](ctx, c, url.String(), operation, params)
		})
	})
}

func getInternal[T any](ctx context.Context, c *BungieClient, url string, operation string, params map[string]any) (BungieHttpResult[T], error) {
	startTime := time.Now()

	fields := map[string]any{
		logging.OPERATION: operation,
		logging.ENDPOINT:  url,
	}
	maps.Copy(fields, params)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		clientLogger.Error("BUNGIE_REQUEST_CREATE_FAILED", err, fields)
		return makeNonStandardHttpResult[T](0), err
	}
	req.Header.Set("X-API-KEY", c.apiKey)

	resp, err := c.httpClient.Do(req)
	duration := time.Since(startTime).Milliseconds()
	if err != nil {
		fields["error"] = err.Error()
		fields[logging.DURATION] = fmt.Sprintf("%dms", duration)
		clientLogger.Debug("BUNGIE_REQUEST_FAILED", fields)
		return makeNonStandardHttpResult[T](0), err
	}

	fields[logging.STATUS_CODE] = resp.StatusCode

	var parseError = BungieResponseParseError{
		Operation:  operation,
		StatusCode: resp.StatusCode,
	}
	// first check if json header is present
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		fields["content_type"] = contentType
		parseError.ContentType = contentType

		// If it's HTML, try to extract the title
		defer resp.Body.Close()
		if strings.Contains(strings.ToLower(contentType), "text/html") {
			if title := extractHTMLTitle(resp.Body); title != "" {
				fields["html_title"] = title
				parseError.Title = title
			}
		} else if strings.Contains(strings.ToLower(contentType), "text/plain") {
			bytearr, ioErr := io.ReadAll(resp.Body)
			if ioErr == nil {
				text := string(bytearr)
				fields["text"] = text
				parseError.Title = text
			}
		}

		if !parseError.isKnownHTMLErrorPage() {
			clientLogger.Warn("BUNGIE_RESPONSE_NOT_PARSABLE", &parseError, fields)

		}
		return makeNonStandardHttpResult[T](resp.StatusCode), &parseError
	}

	if resp.StatusCode != http.StatusOK {
		error_response, err := decodeResponse[BungieError](resp)
		if err != nil {
			fields[logging.DURATION] = fmt.Sprintf("%dms", duration)
			clientLogger.Error("BUNGIE_ERROR_DECODE_FAILED", err, fields)
			return makeNonStandardHttpResult[T](resp.StatusCode), err
		}
		error_response.operation = operation
		result := BungieHttpResult[T]{
			HttpStatusCode:    resp.StatusCode,
			BungieErrorCode:   error_response.ErrorCode,
			BungieErrorStatus: error_response.ErrorStatus,
			Data:              nil,
			MessageData:       error_response.MessageData,
		}
		fields["bungie_error_code"] = error_response.ErrorCode
		fields["bungie_error_status"] = error_response.ErrorStatus
		fields[logging.DURATION] = fmt.Sprintf("%dms", duration)
		clientLogger.Debug("BUNGIE_REQUEST_ERROR", fields)
		return result, error_response
	}

	response, err := decodeResponse[BungieResponse[T]](resp)
	if err != nil {
		fields["error"] = err.Error()
		fields[logging.DURATION] = fmt.Sprintf("%dms", duration)
		clientLogger.Debug("BUNGIE_RESPONSE_DECODE_FAILED", fields)
		return makeNonStandardHttpResult[T](resp.StatusCode), err
	}

	fields["bungie_error_code"] = response.ErrorCode
	fields["bungie_error_status"] = response.ErrorStatus
	fields[logging.DURATION] = fmt.Sprintf("%dms", duration)

	// Invariant violation: HTTP 200 should mean Bungie ErrorCode == Success
	if response.ErrorCode != Success {
		err := fmt.Errorf("invariant violation: HTTP 200 but Bungie ErrorCode=%d (expected %d): %s", response.ErrorCode, Success, response.ErrorStatus)
		clientLogger.Error("BUNGIE_RESPONSE_INVALID_STATE", err, fields)
		return BungieHttpResult[T]{
			HttpStatusCode:    resp.StatusCode,
			BungieErrorCode:   response.ErrorCode,
			BungieErrorStatus: response.ErrorStatus,
			Data:              &response.Response,
			MessageData:       nil,
		}, err
	}

	result := BungieHttpResult[T]{
		HttpStatusCode:    resp.StatusCode,
		BungieErrorCode:   response.ErrorCode,
		BungieErrorStatus: response.ErrorStatus,
		Data:              &response.Response,
		MessageData:       nil,
	}

	clientLogger.Debug("BUNGIE_REQUEST_SUCCESS", fields)

	return result, nil
}

func decodeResponse[T any](resp *http.Response) (*T, error) {
	var data T
	defer resp.Body.Close()
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&data); err != nil {
		return nil, err
	}
	return &data, nil
}

var titleRegex = regexp.MustCompile(`(?i)<title[^>]*>([^<]+)</title>`)

// extractHTMLTitle attempts to extract the title from an HTML response body
// Reads up to 64KB to avoid memory issues
func extractHTMLTitle(body io.ReadCloser) string {
	// Limit read to 64KB
	limitedReader := io.LimitReader(body, 64*1024)
	content, err := io.ReadAll(limitedReader)
	if err != nil {
		return ""
	}

	matches := titleRegex.FindStringSubmatch(string(content))
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

func IsTransientError(bungieErrorCode int, httpStatusCode int) bool {
	switch bungieErrorCode {
	case
		Success,
		CharacterNotFound,
		PGCRNotFound,
		GroupNotFound,
		InvalidParameters,
		DestinyAccountNotFound,
		DestinyPrivacyRestriction:
		return false
	}

	if httpStatusCode == http.StatusBadRequest {
		return false
	}

	// All other errors are considered transient and should be retried
	return true
}

// BungieRetryConfig retries transient network errors for Bungie API calls
// It specifically excludes BungieError instances (application-level errors) from retries
// such as timeout, connection errors, and server errors (5xx)
func BungieRetryConfig() retry.RetryConfig {
	transientRetryConfig := network.TransientNetworkErrorRetryConfig()
	return retry.RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 50 * time.Millisecond,
		MaxDelay:     5 * time.Second,
		Multiplier:   2.0,
		Jitter:       0.1, // 10% jitter
		OnRetry:      nil,
		ShouldRetry: func(err error) bool {
			// Check if this is a Bungie error (application-level error, not a network error)
			// Use errors.As to check through the error chain for wrapped errors
			var bungieErr *BungieError
			if errors.As(err, &bungieErr) && bungieErr.ErrorCode > 1 {
				return false
			}

			return transientRetryConfig.ShouldRetry(err)
		},
	}
}

func (c *BungieClient) makeURL(path string, queryParams netUrl.Values) netUrl.URL {
	rawQuery := ""
	if queryParams != nil {
		rawQuery = queryParams.Encode()
	}
	return netUrl.URL{
		Scheme:   c.scheme,
		Host:     c.host,
		Path:     path,
		RawQuery: rawQuery,
	}
}

func (c *BungieClient) GetPGCR(ctx context.Context, instanceId int64, queryParams netUrl.Values) (BungieHttpResult[DestinyPostGameCarnageReport], error) {
	url := c.makeURL(fmt.Sprintf("/Platform/Destiny2/Stats/PostGameCarnageReport/%d/", instanceId), queryParams)
	return get[DestinyPostGameCarnageReport](ctx, c, url, "get_pgcr", map[string]any{
		logging.INSTANCE_ID: instanceId,
	})
}

func (c *BungieClient) GetProfile(ctx context.Context, membershipType int, membershipId int64, components []int) (BungieHttpResult[DestinyProfileResponse], error) {
	componentsStr := strings.Trim(strings.ReplaceAll(fmt.Sprintf("%v", components), " ", ","), "[]")
	queryParams := netUrl.Values{}
	queryParams.Set("components", componentsStr)
	url := c.makeURL(fmt.Sprintf("/Platform/Destiny2/%d/Profile/%d/", membershipType, membershipId), queryParams)
	return get[DestinyProfileResponse](ctx, c, url, "get_profile", map[string]any{
		logging.MEMBERSHIP_TYPE: membershipType,
		logging.MEMBERSHIP_ID:   membershipId,
		"components":            components,
	})
}

func (c *BungieClient) GetCharacter(ctx context.Context, membershipType int, membershipId int64, characterId int64) (BungieHttpResult[DestinyCharacterResponse], error) {
	queryParams := netUrl.Values{}
	queryParams.Set("components", "200")
	url := c.makeURL(fmt.Sprintf("/Platform/Destiny2/%d/Profile/%d/Character/%d/", membershipType, membershipId, characterId), queryParams)
	return get[DestinyCharacterResponse](ctx, c, url, "get_character", map[string]any{
		logging.MEMBERSHIP_TYPE: membershipType,
		logging.MEMBERSHIP_ID:   membershipId,
		logging.CHARACTER_ID:    characterId,
		"components":            []int{200},
	})
}

func (c *BungieClient) GetDestinyManifest(ctx context.Context) (BungieHttpResult[DestinyManifest], error) {
	url := c.makeURL("/Platform/Destiny2/Manifest/", nil)
	return get[DestinyManifest](ctx, c, url, "get_destiny_manifest", nil)
}

func (c *BungieClient) GetHistoricalStats(ctx context.Context, membershipType int, membershipId int64) (BungieHttpResult[DestinyHistoricalStatsAccountResult], error) {
	url := c.makeURL(fmt.Sprintf("/Platform/Destiny2/%d/Account/%d/Stats/", membershipType, membershipId), nil)
	return get[DestinyHistoricalStatsAccountResult](ctx, c, url, "get_historical_stats", map[string]any{
		logging.MEMBERSHIP_TYPE: membershipType,
		logging.MEMBERSHIP_ID:   membershipId,
	})
}

func (c *BungieClient) GetLinkedProfiles(ctx context.Context, membershipType int, membershipId int64, getAllMemberships bool) (BungieHttpResult[LinkedProfiles], error) {
	queryParams := netUrl.Values{}
	queryParams.Set("getAllMemberships", fmt.Sprintf("%t", getAllMemberships))
	url := c.makeURL(fmt.Sprintf("/Platform/Destiny2/%d/Profile/%d/LinkedProfiles/", membershipType, membershipId), queryParams)
	return get[LinkedProfiles](ctx, c, url, "get_linked_profiles", map[string]any{
		logging.MEMBERSHIP_TYPE: membershipType,
		logging.MEMBERSHIP_ID:   membershipId,
	})
}

func (c *BungieClient) GetGroup(ctx context.Context, groupId int64) (BungieHttpResult[GroupResponse], error) {
	url := c.makeURL(fmt.Sprintf("/Platform/GroupV2/%d/", groupId), nil)
	return get[GroupResponse](ctx, c, url, "get_group", map[string]any{
		logging.GROUP_ID: groupId,
	})
}

func (c *BungieClient) GetGroupsForMember(ctx context.Context, membershipType int, membershipId int64) (BungieHttpResult[GetGroupsForMemberResponse], error) {
	url := c.makeURL(fmt.Sprintf("/Platform/GroupV2/User/%d/%d/0/1/", membershipType, membershipId), nil)
	return get[GetGroupsForMemberResponse](ctx, c, url, "get_groups_for_member", map[string]any{
		logging.MEMBERSHIP_TYPE: membershipType,
		logging.MEMBERSHIP_ID:   membershipId,
	})
}

func (c *BungieClient) GetMembersOfGroup(ctx context.Context, groupId int64, page int) (BungieHttpResult[SearchResultOfGroupMember], error) {
	queryParams := netUrl.Values{}
	queryParams.Set("currentpage", fmt.Sprintf("%d", page))
	queryParams.Set("memberType", "0")
	url := c.makeURL(fmt.Sprintf("/Platform/GroupV2/%d/Members/", groupId), queryParams)
	return get[SearchResultOfGroupMember](ctx, c, url, "get_members_of_group", map[string]any{
		logging.GROUP_ID: groupId,
		"page":           page,
	})
}

func (c *BungieClient) GetCommonSettings(ctx context.Context) (BungieHttpResult[CoreSettingsConfiguration], error) {
	url := c.makeURL("/Platform/Settings/", nil)
	return get[CoreSettingsConfiguration](ctx, c, url, "get_common_settings", nil)
}

func (c *BungieClient) GetActivityHistoryPage(ctx context.Context, membershipType int, membershipId int64, characterId int64, count int, page int, mode int) (BungieHttpResult[DestinyActivityHistoryResults], error) {
	queryParams := netUrl.Values{}
	queryParams.Set("mode", fmt.Sprintf("%d", mode))
	queryParams.Set("count", fmt.Sprintf("%d", count))
	queryParams.Set("page", fmt.Sprintf("%d", page))
	url := c.makeURL(fmt.Sprintf("/Platform/Destiny2/%d/Account/%d/Character/%d/Stats/Activities/", membershipType, membershipId, characterId), queryParams)
	return get[DestinyActivityHistoryResults](ctx, c, url, "get_activity_history_page", map[string]any{
		logging.MEMBERSHIP_TYPE: membershipType,
		logging.MEMBERSHIP_ID:   membershipId,
		logging.CHARACTER_ID:    characterId,
		"mode":                  mode,
		"count":                 count,
		"page":                  page,
	})
}
