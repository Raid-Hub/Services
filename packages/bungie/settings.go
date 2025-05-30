package bungie

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

type GetCommonSettingsResponse struct {
	Response        CoreSettingsConfiguration `json:"Response"`
	ErrorCode       int                       `json:"ErrorCode"`
	ErrorStatus     string                    `json:"ErrorStatus"`
	ThrottleSeconds int                       `json:"ThrottleSeconds"`
}

type CoreSettingsConfiguration struct {
	Systems map[string]CoreSystem `json:"systems"`
}

type CoreSystem struct {
	Enabled bool `json:"enabled"`
}

func GetCommonSettings() (*CoreSettingsConfiguration, error) {
	url := fmt.Sprintf("%s/Platform/Settings", getBungieURL())
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	apiKey := os.Getenv("BUNGIE_API_KEY") // Read the API key from the BUNGIE_API_KEY environment variable
	req.Header.Set("X-API-Key", apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	decoder := json.NewDecoder(resp.Body)

	if resp.StatusCode != http.StatusOK {
		var data BungieError
		if err := decoder.Decode(&data); err != nil {
			return nil, err
		}

		return nil, fmt.Errorf("error response: %s (%d)", data.Message, data.ErrorCode)
	}

	var data GetCommonSettingsResponse
	if err := decoder.Decode(&data); err != nil {
		return nil, err
	}

	return &data.Response, nil
}
