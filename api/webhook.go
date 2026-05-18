package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

var mu sync.RWMutex

var allowedUsers = map[int64]bool{
	1037388537: true,
	1453436329: true,
}

var quotes = []string{
	"Антон, ау! Уже 19:00, а стрима нет. Всё норм? 👀",
	"Хонести, твои зрители начинают думать, что ты миф 🐉",
	"Антон, стрим сам себя не запустит... или запустит? 🤔",
	"Хонести! Люди уже пришли с работы и ждут тебя 🍕",
	"Антон, холодильник проверил, стрима там нет. Может на твиче? 😄",
	"Хонести, зрители уже чай заварили и ждут... 🍵",
	"Антон, сегодня стрим будет? Спрашиваю для друга 👀",
	"Хонести! Твич пыль собирает, заходи 🧹",
	"Антон, мы знаем что ты там. Выходи стримить 😏",
	"Хонести, зрители волнуются. Дай знак что жив 👋",
	"Антон! Уже вечер, самое время для стрима 🌙",
	"Хонести, без тебя скучно. Давай уже 🥺",
	"Антон, стрим — это витамины. Принимай ежедневно 💊",
	"Хонести! Кто-то сказал что сегодня будет стрим... был ли это ты? 🤷",
	"Антон, твич тебя ждёт как старый друг 🤝",
	"Хонести, зрители уже расположились на диване — осталось только ты 🛋️",
	"Антон! Говорят лучшие стримы начинаются после 19:00 👌",
	"Хонести, без стрима вечер не считается 📺",
	"Антон, ты же не забыл что у тебя есть твич? 😅",
	"Хонести! Стрим — лучшее лекарство от скуки. Для нас 😄",
	"Антон, зрители уже греют место перед монитором 💺",
	"Хонести, говорят прямой эфир продлевает жизнь. Проверь! 😂",
	"Антон! Всё готово к стриму — зрители, чай, снеки. Ждём тебя 🎮",
	"Хонести, сегодня вечер пятницы. Лучшее время для стрима! 🎉",
	"Антон, без стрима вечер как без соли 🧂",
	"Хонести! Зрители смотрят на часы каждые 5 минут ⏰",
	"Антон, ты наша любимая передача. Начинай вещание 📡",
	"Хонести, стрим это не обязанность, это призвание. Откликнись! 🦸",
	"Антон! Говорят стримеры которые стримят вечером — самые счастливые 😊",
	"Хонести, где стрим? Спрашивает вся теплая компания зрителей 🫂",
	"Антон, монитор без твоего лица скучает 🖥️",
	"Хонести! Уже вечер — самое время показать всем как ты играешь 🎯",
	"Антон, зрители готовы. Микрофон готов. Ты следующий! 🎤",
	"Хонести, стрим сегодня? Или ждём до завтра? 😴",
	"Антон! Без стрима вечер какой-то неполный 🌑",
	"Хонести, ты лучший стример которого мы знаем. Докажи это сегодня 💪",
	"Антон, зрители уже разогрелись — давай выходи 🔥",
	"Хонести! Вечерний стрим — это традиция. Уважай традиции 🏛️",
	"Антон, твич без тебя как праздник без торта 🎂",
	"Хонести, сегодня отличный день для стрима. Как и вчера. И завтра тоже будет 😄",
	"Антон! Стримить или не стримить — вот в чём вопрос. Ответ: стримить 🎭",
	"Хонести, зрители уже написали в календаре: стрим сегодня вечером ✅",
	"Антон, все дороги ведут на твой стрим 🗺️",
	"Хонести! Лучший способ провести вечер — стримить. Мы проверили 😎",
	"Антон, запусти стрим — сделай наш вечер лучше 🌟",
	"Хонести, ты же знаешь что мы ждём? Правильно — стрим! 👏",
	"Антон! Говорят вечерние стримы приносят удачу. Проверяй! 🍀",
	"Хонести, зрители — народ терпеливый. Но не бесконечно 😇",
	"Антон, включай стрим — поговорим как нормальные люди 💬",
	"Хонести! Мы верим в тебя. Особенно когда ты стримишь 🙌",
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

func redisSetEx(key, value string, ttlSeconds int) error {
	token := os.Getenv("UPSTASH_REDIS_REST_TOKEN")
	restURL := os.Getenv("UPSTASH_REDIS_REST_URL")

	cmd := []interface{}{"SET", key, value, "EX", ttlSeconds}
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

func isStreamLive() bool {
	clientID := os.Getenv("TWITCH_CLIENT_ID")
	clientSecret := os.Getenv("TWITCH_CLIENT_SECRET")

	tokenURL := "https://id.twitch.tv/oauth2/token"
	tokenBody := fmt.Sprintf("client_id=%s&client_secret=%s&grant_type=client_credentials", clientID, clientSecret)
	tokenResp, err := http.Post(tokenURL, "application/x-www-form-urlencoded", strings.NewReader(tokenBody))
	if err != nil {
		return false
	}
	defer tokenResp.Body.Close()

	var tokenData struct {
		AccessToken string `json:"access_token"`
	}
	json.NewDecoder(tokenResp.Body).Decode(&tokenData)
	if tokenData.AccessToken == "" {
		return false
	}

	req, err := http.NewRequest("GET", "https://api.twitch.tv/helix/streams?user_login=honesty113", nil)
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer "+tokenData.AccessToken)
	req.Header.Set("Client-Id", clientID)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	var streamData struct {
		Data []struct {
			Type string `json:"type"`
		} `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&streamData)

	return len(streamData.Data) > 0 && streamData.Data[0].Type == "live"
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
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	payload := fmt.Sprintf(
		`{"chat_id":%s,"text":%s,"parse_mode":"HTML","disable_web_page_preview":true}`,
		jsonString(fmt.Sprintf("%v", chatID)), jsonString(text),
	)
	resp, err := http.Post(apiURL, "application/json", strings.NewReader(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func sendTelegramTo(botToken string, chatID interface{}, text string) error {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	payload := fmt.Sprintf(
		`{"chat_id":%s,"text":%s,"parse_mode":"HTML","disable_web_page_preview":true}`,
		jsonString(fmt.Sprintf("%v", chatID)), jsonString(text),
	)
	resp, err := http.Post(apiURL, "application/json", strings.NewReader(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func sendReplyKeyboard(botToken string, chatID interface{}, text string) error {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	keyboard := `{"keyboard":[[{"text":"▶ Старт"},{"text":"⏹ Стоп"}],[{"text":"📋 Сообщение"}]],"resize_keyboard":true,"persistent":true}`
	payload := fmt.Sprintf(
		`{"chat_id":%s,"text":%s,"parse_mode":"HTML","disable_web_page_preview":true,"reply_markup":%s}`,
		jsonString(fmt.Sprintf("%v", chatID)), jsonString(text), keyboard,
	)
	resp, err := http.Post(apiURL, "application/json", strings.NewReader(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func sendUpdateNotification(botToken string, chatID interface{}, title, category string) error {
	redisSetEx("pending_title", title, 30*24*60*60)
	redisSetEx("pending_category", category, 30*24*60*60)

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	text := fmt.Sprintf("🔔 Смена категории и названия\n\nКатегория: <b>%s</b>\nНазвание: <b>%s</b>\n\nОповестить подписчиков?", category, title)
	buttons := `{"inline_keyboard":[[{"text":"📣 Оповестить","callback_data":"notify"},{"text":"❌ Не надо","callback_data":"skip"}]]}`
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

func handleCron(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Cron-Secret") != os.Getenv("CRON_SECRET") {
		http.Error(w, "unauthorized", http.StatusForbidden)
		return
	}

	now := time.Now().UTC().Add(3 * time.Hour)
	hour := now.Hour()

	if hour < 19 || hour >= 21 {
		w.WriteHeader(http.StatusOK)
		return
	}

	if isStreamLive() {
		w.WriteHeader(http.StatusOK)
		return
	}

	sent := redisGet("quote_sent_today")
	if sent == today() {
		w.WriteHeader(http.StatusOK)
		return
	}

	if rand.Float32() < 0.4 {
		w.WriteHeader(http.StatusOK)
		return
	}

	redisSet("quote_sent_today", today())

	quote := quotes[rand.Intn(len(quotes))]
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	for userID := range allowedUsers {
		sendTelegramTo(botToken, userID, quote)
	}

	w.WriteHeader(http.StatusOK)
}

func today() string {
	return time.Now().UTC().Add(3 * time.Hour).Format("2006-01-02")
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
		case cq.Data == "skip":
			sendTelegramTo(botToken, chatID, "👌 Оповещение отменено")
		case cq.Data == "notify":
			category := redisGet("pending_category")
			title := redisGet("pending_title")
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

	switch text {
	case "/start":
		sendReplyKeyboard(botToken, chatID, statusText())
	case "▶ Старт":
		status := redisGet("bot_status")
		if status == "off" {
			redisSet("bot_status", "on")
			sendReplyKeyboard(botToken, chatID, "✅ Бот запущен\n\n"+statusText())
		} else {
			sendReplyKeyboard(botToken, chatID, "⚠️ Бот уже запущен\n\n"+statusText())
		}
	case "⏹ Стоп":
		redisSet("bot_status", "off")
		sendReplyKeyboard(botToken, chatID, "🛑 Бот остановлен\n\n"+statusText())
	case "📋 Сообщение":
		msg := redisGet("stream_message")
		if msg == "" {
			msg = "Сообщение не задано"
		}
		sendReplyKeyboard(botToken, chatID, "📋 Текущее сообщение:\n\n"+msg+"\n\n"+statusText())
	default:
		if text == "" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if err := redisSet("stream_message", text); err != nil {
			sendTelegramTo(botToken, chatID, "Ошибка сохранения.")
			w.WriteHeader(http.StatusOK)
			return
		}
		sendReplyKeyboard(botToken, chatID, fmt.Sprintf("✅ Сообщение обновлено:\n\n%s\n\n%s", text, statusText()))
	}

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
	case "/cron":
		handleCron(w, r)
	default:
		http.NotFound(w, r)
	}
}
