package gm_report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"raidhub/lib/env"
	"time"
)

type GmReportWebhook struct {
	Id       int64                   `json:"id"`
	Player   int64                   `json:"player,string"`
	Metadata GmReportWebhookMetadata `json:"metadata"`
}

type GmReportWebhookMetadata struct {
	CheaterAccountProbability  float64              `json:"cheater_account_probability"`
	CheaterAccountHeuristics   []string             `json:"cheater_account_heuristics"`
	RaidHubCheatLevel          int                  `json:"raidhub_cheat_level"`
	EstimatedAccountAgeDays    float64              `json:"estimated_account_age_days"`
	LookBackDays               int                  `json:"look_back_days"`
	RaidClears                 int                  `json:"raid_clears"`
	FractionRaidClearsSolo     float64              `json:"fraction_raid_clears_solo"`
	FractionRaidClearsLowman   float64              `json:"fraction_raid_clears_lowman"`
	FractionRaidClearsFlawless float64              `json:"fraction_raid_clears_flawless"`
	Flags                      GmReportWebhookFlags `json:"flags"`
	LastSeen                   time.Time            `json:"last_seen"`
}
type GmReportWebhookFlags struct {
	Total  int `json:"total"`
	ClassA int `json:"class_a"`
	ClassB int `json:"class_b"`
	ClassC int `json:"class_c"`
	ClassD int `json:"class_d"`
}

func SendGmReportWebhook(membershipId int64, metadata GmReportWebhookMetadata) error {
	webhookUrl := env.GMReportWebhookURL

	webhookData := GmReportWebhook{
		Id:       (membershipId << 20) + int64(metadata.RaidHubCheatLevel),
		Player:   membershipId,
		Metadata: metadata,
	}

	body, err := json.Marshal(webhookData)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook data: %w", err)
	}

	authorization := fmt.Sprintf("App %s", env.GMReportWebhookAuth)

	req, err := http.NewRequest("POST", webhookUrl, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authorization)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("webhook request failed with status: %s", resp.Status)
	}

	return nil
}
