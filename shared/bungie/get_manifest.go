package bungie

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"math/rand"
)

type DestinyManifestResponse struct {
	Response        DestinyManifest `json:"Response"`
	ErrorCode       int             `json:"ErrorCode"`
	ErrorStatus     string          `json:"ErrorStatus"`
	ThrottleSeconds int             `json:"ThrottleSeconds"`
}

type DestinyManifest struct {
	JsonWorldComponentContentPaths map[string]map[string]string `json:"jsonWorldComponentContentPaths"`
	JsonWorldContentPaths          map[string]string            `json:"jsonWorldContentPaths"`
	MobileWorldContentPaths        map[string]string            `json:"mobileWorldContentPaths"`
	Version                        string                       `json:"version"`
}

func GetDestinyManifest() (*DestinyManifest, error) {
	url := fmt.Sprintf("https://www.bungie.net/Platform/Destiny2/Manifest/?c=%d", rand.Int())
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	apiKey := os.Getenv("BUNGIE_API_KEY")
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

	var data DestinyManifestResponse
	if err := decoder.Decode(&data); err != nil {
		return nil, err
	}

	return &data.Response, nil
}
