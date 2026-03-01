package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

type balanceResponse struct {
	BalanceInfos []struct {
		Currency     string `json:"currency"`
		TotalBalance string `json:"total_balance"`
		GrantedBalance string `json:"granted_balance"`
		ToppedUpBalance string `json:"topped_up_balance"`
	} `json:"balance_infos"`
}

// showUsage queries and prints balance info for the current provider.
func showUsage(prov string) {
	switch prov {
	case "deepseek":
		apiKey := os.Getenv("DEEPSEEK_API_KEY")
		if apiKey == "" {
			cfg := loadConfig()
			apiKey = cfg["DEEPSEEK_API_KEY"]
		}
		if apiKey == "" {
			fmt.Println("No DEEPSEEK_API_KEY configured")
			return
		}
		client := &http.Client{Timeout: 5 * time.Second}
		req, _ := http.NewRequest("GET", "https://api.deepseek.com/user/balance", nil)
		req.Header.Set("Authorization", "Bearer "+apiKey)
		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		defer resp.Body.Close()
		var b balanceResponse
		if err := json.NewDecoder(resp.Body).Decode(&b); err != nil || len(b.BalanceInfos) == 0 {
			fmt.Println("Failed to query balance")
			return
		}
		info := b.BalanceInfos[0]
		fmt.Printf("DeepSeek balance: ¥%s (granted: ¥%s, topped-up: ¥%s)\n",
			info.TotalBalance, info.GrantedBalance, info.ToppedUpBalance)
	case "gemini":
		fmt.Println("Gemini: free tier (no balance to query)")
	case "anthropic":
		fmt.Println("Anthropic: check usage at https://console.anthropic.com/")
	default:
		fmt.Printf("No usage info for provider: %s\n", prov)
	}
}

