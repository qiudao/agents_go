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

// showUsage queries and prints balance info for all configured providers.
func showUsage(prov string) {
	cfg := loadConfig()
	showed := false

	// DeepSeek
	dsKey := os.Getenv("DEEPSEEK_API_KEY")
	if dsKey == "" {
		dsKey = cfg["DEEPSEEK_API_KEY"]
	}
	if dsKey != "" {
		showed = true
		queryBalance("DeepSeek", "https://api.deepseek.com/user/balance", dsKey)
	}

	// Qwen/百炼
	qwKey := os.Getenv("QWEN_API_KEY")
	if qwKey == "" {
		qwKey = cfg["QWEN_API_KEY"]
	}
	if qwKey != "" {
		showed = true
		fmt.Println("Qwen:     https://bailian.console.aliyun.com/")
	}

	// Gemini
	gmKey := os.Getenv("GEMINI_API_KEY")
	if gmKey == "" {
		gmKey = cfg["GEMINI_API_KEY"]
	}
	if gmKey != "" {
		showed = true
		fmt.Println("Gemini:   free tier")
	}

	// Anthropic
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		showed = true
		fmt.Println("Anthropic: https://console.anthropic.com/")
	}

	if !showed {
		fmt.Printf("No API keys configured (current provider: %s)\n", prov)
	}
}

func queryBalance(name, url, apiKey string) {
	client := &http.Client{Timeout: 5 * time.Second}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("%s: error %v\n", name, err)
		return
	}
	defer resp.Body.Close()

	var b balanceResponse
	if err := json.NewDecoder(resp.Body).Decode(&b); err != nil || len(b.BalanceInfos) == 0 {
		fmt.Printf("%s: unable to query balance\n", name)
		return
	}
	info := b.BalanceInfos[0]
	fmt.Printf("%s: %s%s (granted: %s, topped-up: %s)\n",
		name, currencySymbol(info.Currency), info.TotalBalance, info.GrantedBalance, info.ToppedUpBalance)
}

func currencySymbol(c string) string {
	switch c {
	case "CNY":
		return "¥"
	case "USD":
		return "$"
	default:
		return c + " "
	}
}

