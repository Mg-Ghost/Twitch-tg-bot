package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
)

var (
	customMessage string
	mu            sync.RWMutex
)

type TwitchEvent struct {
	BroadcasterUserLogin string `json:"broadcaster_user_login"`
	BroadcasterUserName  string `json:"broadcaster_user_name"`
}

type TwitchPayload struct {
	Challenge    string `json:"challenge"`
	Subscription struct {
		Type string `json:"type"`
	} `json:"subscription"`
	Event TwitchEvent `json:"event"`
}

type SetMessageRequest struct {
	Message string `json:"message"`
	Token   string `json:"token"`
}

func verifyTwitchSignature(r *http.Request, body []byte) bool {
	secret := os.Getenv("TWITCH_WEBHOOK_SECRET")
	msgID := r.Header.Get("Twitch-Eventsub-Message-Id")
	timestamp := r.Header.Get("Twitch-Eventsub-Message-Timestamp")
	signature := r.Header.Get("Twitch-Eventsub-Message-Signature")

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(msgID + timestamp + string(body)))
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expected), []byte(signature))
}

func sendTelegramMessage(text string) error {
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID := os.Getenv("TELEGRAM_CHANNEL_ID")

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	payload := fmt.Sprintf(
		`{"chat_id":"%s","text":"%s","parse_mode":"HTML"}`,
		chatID, text,
	)

	resp, err := http.Post(url, "application/json", strings.NewReader(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func Handler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost && r.URL.Path == "/set-message" {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "cannot read body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		var req SetMessageRequest
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		if req.Token != os.Getenv("BOT_ADMIN_TOKEN") {
			http.Error(w, "unauthorized", http.StatusForbidden)
			return
		}

		mu.Lock()
		customMessage = req.Message
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true,"message":"сообщение обновлено"}`))
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "cannot read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if !verifyTwitchSignature(r, body) {
		http.Error(w, "invalid signature", http.StatusForbidden)
		return
	}

	var payload TwitchPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	msgType := r.Header.Get("Twitch-Eventsub-Message-Type")
	if msgType == "webhook_callback_verification" {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(payload.Challenge))
		return
	}

	if payload.Subscription.Type != "stream.online" {
		w.WriteHeader(http.StatusOK)
		return
	}

	mu.RLock()
	msg := customMessage
	mu.RUnlock()

	var text string
	if msg != "" {
		text = msg
	} else {
		streamer := payload.Event.BroadcasterUserLogin
		displayName := payload.Event.BroadcasterUserName
		if displayName == "" {
			displayName = streamer
		}
		text = fmt.Sprintf(
			"🔴 <b>%s</b> начал стрим!\n\nЗаходите смотреть: https://twitch.tv/%s",
			displayName, streamer,
		)
	}

	if err := sendTelegramMessage(text); err != nil {
		http.Error(w, "telegram error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}