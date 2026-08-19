package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func main() {
	// 1. Login
	loginReq := map[string]string{
		"username": "kanchanaedeAPI",
		"password": "KanchanaBro#999$",
	}
	b, _ := json.Marshal(loginReq)

	fmt.Println("--- LOGIN ---")
	resp, err := http.Post("https://esms.dialog.lk/api/v2/user/login", "application/json", bytes.NewReader(b))
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	fmt.Println("Status:", resp.StatusCode)
	fmt.Println("Body:", string(body))

	var loginResp struct {
		Token string `json:"token"`
	}
	json.Unmarshal(body, &loginResp)

	if loginResp.Token == "" {
		fmt.Println("No token!")
		return
	}

	// 2. Send SMS to a dummy number to see response format
	smsReq := map[string]interface{}{
		"msisdn": []map[string]string{
			{"mobile": "777777777"},
		},
		"sourceAddress": "Test Kanch",
		"message":       "Test message",
		"transaction_id": 9999999999,
		"payment_method": 0,
	}
	b2, _ := json.Marshal(smsReq)

	fmt.Println("\n--- SEND SMS ---")
	req2, _ := http.NewRequest("POST", "https://e-sms.dialog.lk/api/v2/sms", bytes.NewReader(b2))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer "+loginResp.Token)

	client := &http.Client{}
	resp2, err := client.Do(req2)
	if err != nil {
		panic(err)
	}
	defer resp2.Body.Close()
	body2, _ := io.ReadAll(resp2.Body)
	fmt.Println("Status:", resp2.StatusCode)
	fmt.Println("Body:", string(body2))
}
