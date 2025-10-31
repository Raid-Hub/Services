package bungie

import (
	"fmt"
	"net/http"
	"raidhub/lib/env"
	"time"
)

var (
	PGCRClient *BungieClient
	Client     *BungieClient
)

func init() {
	clientLogger.Info("BUNGIE_CLIENT_INITIALIZED", map[string]any{
		"host": env.ZeusHost,
		"port": env.ZeusPort,
	})
	httpClient := &http.Client{
		Timeout: 15 * time.Second,
	}
	zeusURL := fmt.Sprintf("http://%s:%s", env.ZeusHost, env.ZeusPort)
	Client = &BungieClient{
		httpClient: httpClient,
		baseURL:    zeusURL,
		apiKey:     env.BungieAPIKey,
	}
	PGCRClient = &BungieClient{
		httpClient: httpClient,
		baseURL:    zeusURL,
		apiKey:     env.BungieAPIKey,
	}
}
