package bungie

import (
	"net/http"
	"raidhub/lib/env"
	"time"
)

var (
	PGCRClient *BungieClient
	Client     *BungieClient
)

func init() {
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}
	Client = &BungieClient{
		httpClient: httpClient,
		baseURL:    env.BungieURLBase,
		apiKey:     env.BungieAPIKey,
	}
	PGCRClient = &BungieClient{
		httpClient: httpClient,
		baseURL:    env.PGCRURLBase,
		apiKey:     env.BungieAPIKey,
	}
}
