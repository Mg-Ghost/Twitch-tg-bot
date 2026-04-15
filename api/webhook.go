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
	Title                string `json:"title"`
	CategoryName         string `json:"category_name"`
}

type TwitchPayload struct {
	Challenge    string `json:"challenge"`
	Subscription struct {
		Type string `json:"type"`
	} `json:"subscription"`
	Event TwitchEvent `json:"event"`
}

type TelegramUpdate struct {
	Message *struct {
		Text string `json:"text"`
		From struct {
			ID int64 `json:"id"`
		} `json:"from"`
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
	} `json:"message"`
	CallbackQuery *struct {
		ID   string `json:"id"`
		Data string `json:"data"`
		From struct {
			ID int64 `json:"id"`
		} `json:"from"`
		Message struct {
			Chat struct {
				ID int64 `json:"id"`
			} `json:"chat"`
		} `json:"message"`
	} `json:"callback_query"`
}

func redisSet(key, value string) error {
	token := os.Getenv("UPSTASH_REDIS_REST_TOKEN")
	restURL := os.Getenv("UPSTASH_REDIS_REST_URL")

	cmd := []interface{}{"SET", key, value}
	body, _ := json.Marshal(cmd)

	req, err := http.NewRequest("POST", restURL, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func redisGet(key string) string {
	token := os.Getenv("UPSTASH_REDIS_REST_TOKEN")
	restURL := os.Getenv("UPSTASH_REDIS_REST_URL")

	cmd := []interface{}{"GET", key}
	body, _ := json.Marshal(cmd)

	req, err := http.NewRequest("POST", restURL, strings.NewReader(string(body)))
	if err != nil {
		return ""
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

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
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	payload := fmt.Sprintf(
		`{"chat_id":%s,"text":%s,"parse_mode":"HTML"}`,
		jsonString(fmt.Sprintf("%v", chatID)), jsonString(text),
	)
	resp, err := http.Post(apiURL, "application/json", strings.NewReader(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func sendWithButtons(botToken string, chatID interface{}, text string) error {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	buttons := `{"inline_keyboard":[[{"text":"▶ Старт","callback_data":"start"},{"text":"⏹ Стоп","callback_data":"stop"}],[{"text":"📋 Сообщение","callback_data":"message"}]]}`
	payload := fmt.Sprintf(
		`{"chat_id":%s,"text":%s,"parse_mode":"HTML","reply_markup":%s}`,
		jsonString(fmt.Sprintf("%v", chatID)), jsonString(text), buttons,
	)
	resp, err := http.Post(apiURL, "application/json", strings.NewReader(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func sendUpdateNotification(botToken string, chatID interface{}, title, category string) error {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	text := fmt.Sprintf("🔔 Смена категории и названия\n\nКатегория: <b>%s</b>\nНазвание: <b>%s</b>\n\nОповестить подписчиков?", category, title)
	buttons := fmt.Sprintf(
		`{"inline_keyboard":[[{"text":"📣 Оповестить","callback_data":"notify_%s_%s"},{"text":"❌ Не надо","callback_data":"skip"}]]}`,
		escapeCallback(category), escapeCallback(title),
	)
	payload := fmt.Sprintf(
		`{"chat_id":%s,"text":%s,"parse_mode":"HTML","reply_markup":%s}`,
		jsonString(fmt.Sprintf("%v", chatID)), jsonString(text), buttons,
	)
	resp, err := http.Post(apiURL, "application/json", strings.NewReader(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func escapeCallback(s string) string {
	s = strings.ReplaceAll(s, "_", "-")
	s = strings.ReplaceAll(s, "|", "-")
	if len(s) > 30 {
		s = s[:30]
	}
	return s
}

func answerCallback(botToken, callbackID string) {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/answerCallbackQuery", botToken)
	payload := fmt.Sprintf(`{"callback_query_id":"%s"}`, callbackID)
	resp, _ := http.Post(apiURL, "application/json", strings.NewReader(payload))
	if resp != nil {
		resp.Body.Close()
	}
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func statusText() string {
	status := redisGet("bot_status")
	if status == "off" {
		return "🔴 Бот остановлен"
	}
	return "🟢 Бот работает"
}

func notifyAllAdmins(botToken, text string) {
	for userID := range allowedUsers {
		sendTelegramTo(botToken, userID, text)
	}
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

	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")

	if update.CallbackQuery != nil {
		cq := update.CallbackQuery
		answerCallback(botToken, cq.ID)

		if !allowedUsers[cq.From.ID] {
			w.WriteHeader(http.StatusOK)
			return
		}

		chatID := cq.Message.Chat.ID

		switch {
		case cq.Data == "start":
			status := redisGet("bot_status")
			if status == "off" {
				redisSet("bot_status", "on")
				sendWithButtons(botToken, chatID, "✅ Бот запущен\n\n"+statusText())
			} else {
				sendWithButtons(botToken, chatID, "⚠️ Бот уже запущен\n\n"+statusText())
			}
		case cq.Data == "stop":
			redisSet("bot_status", "off")
			sendWithButtons(botToken, chatID, "🛑 Бот остановлен\n\n"+statusText())
		case cq.Data == "message":
			msg := redisGet("stream_message")
			if msg == "" {
				msg = "Сообщение не задано"
			}
			sendWithButtons(botToken, chatID, "📋 Текущее сообщение:\n\n"+msg+"\n\n"+statusText())
		case cq.Data == "skip":
			sendTelegramTo(botToken, chatID, "👌 Оповещение отменено")
		case strings.HasPrefix(cq.Data, "notify_"):
			parts := strings.SplitN(strings.TrimPrefix(cq.Data, "notify_"), "_", 2)
			category := ""
			title := ""
			if len(parts) == 2 {
				category = parts[0]
				title = parts[1]
			}
			text := fmt.Sprintf("🔄 Сменили категорию и название!\n\nКатегория: <b>%s</b>\nСидим: <b>%s</b>", category, title)
			sendTelegramMessage(text)
			sendTelegramTo(botToken, chatID, "✅ Оповещение отправлено в канал")
		}

		w.WriteHeader(http.StatusOK)
		return
	}

	if update.Message == nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	userID := update.Message.From.ID
	chatID := update.Message.Chat.ID
	text := update.Message.Text

	if !allowedUsers[userID] {
		sendTelegramTo(botToken, chatID, "У тебя нет доступа.")
		w.WriteHeader(http.StatusOK)
		return
	}

	if text == "/start" {
		sendWithButtons(botToken, chatID, statusText())
		w.WriteHeader(http.StatusOK)
		return
	}

	if text == "" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if err := redisSet("stream_message", text); err != nil {
		sendTelegramTo(botToken, chatID, "Ошибка сохранения.")
		w.WriteHeader(http.StatusOK)
		return
	}

	sendWithButtons(botToken, chatID, fmt.Sprintf("Сообщение обновлено:\n%s\n\n%s", text, statusText()))
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

	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")

	switch payload.Subscription.Type {
	case "stream.online":
		status := redisGet("bot_status")
		if status == "off" {
			w.WriteHeader(http.StatusOK)
			return
		}

		msg := redisGet("stream_message")
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
		sendTelegramMessage(text)

	case "channel.update":
		title := payload.Event.Title
		category := payload.Event.CategoryName
		for userID := range allowedUsers {
			sendUpdateNotification(botToken, userID, title, category)
		}
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
