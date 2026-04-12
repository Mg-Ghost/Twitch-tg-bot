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

var mu sync.RWMutex

var allowedUsers = map[int64]bool{
	1037388537: true,
	1453436329: true,
}

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

type TelegramUpdate struct {
	Message struct {
		Text string `json:"text"`
		From struct {
			ID int64 `json:"id"`
		} `json:"from"`
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
	} `json:"message"`
}

func redisGet() string {
	token := os.Getenv("UPSTASH_REDIS_REST_TOKEN")
	restURL := os.Getenv("UPSTASH_REDIS_REST_URL")

	url := restURL + "/get/stream_message"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	var result struct {
		Result *string `json:"result"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	if result.Result == nil {
		return ""
	}
	return *result.Result
}

func redisSet(value string) error {
	token := os.Getenv("UPSTASH_REDIS_REST_TOKEN")
	restURL := os.Getenv("UPSTASH_REDIS_REST_URL")

	encoded := strings.ReplaceAll(value, " ", "%20")
	url := fmt.Sprintf("%s/set/stream_message/%s", restURL, encoded)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
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
	return sendTelegramTo(botToken, chatID, text)
}

func sendTelegramTo(botToken string, chatID interface{}, text string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	payload := fmt.Sprintf(
		`{"chat_id":%s,"text":%s,"parse_mode":"HTML"}`,
		jsonString(fmt.Sprintf("%v", chatID)), jsonString(text),
	)
	resp, err := http.Post(url, "application/json", strings.NewReader(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func handleTgUpdate(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "cannot read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var update TelegramUpdate
	if err := json.Unmarshal(body, &update); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	userID := update.Message.From.ID
	chatID := update.Message.Chat.ID
	text := update.Message.Text
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")

	if !allowedUsers[userID] {
		sendTelegramTo(botToken, chatID, "У тебя нет доступа.")
		w.WriteHeader(http.StatusOK)
		return
	}

	if text == "" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if err := redisSet(text); err != nil {
		sendTelegramTo(botToken, chatID, "Ошибка сохранения.")
		w.WriteHeader(http.StatusOK)
		return
	}

	sendTelegramTo(botToken, chatID, "Сообщение обновлено")
	w.WriteHeader(http.StatusOK)
}

func handleWebhook(w http.ResponseWriter, r *http.Request) {
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

	msg := redisGet()

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

func Handler(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/tg-update":
		handleTgUpdate(w, r)
	case "/webhook":
		handleWebhook(w, r)
	default:
		http.NotFound(w, r)
	}
}