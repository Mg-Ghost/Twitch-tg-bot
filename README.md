# Twitch → Telegram Bot

Бот отслеживает запуск стрима на Twitch и отправляет уведомление в Telegram канал. Развёрнут на Vercel.

## Стек

Go · Vercel · Twitch EventSub · Telegram Bot API · Upstash Redis

## Переменные окружения

| Переменная | Описание |
|---|---|
| `TELEGRAM_BOT_TOKEN` | Токен от @BotFather |
| `TELEGRAM_CHANNEL_ID` | `@название_канала` |
| `TWITCH_WEBHOOK_SECRET` | Любая строка |
| `UPSTASH_REDIS_REST_URL` | URL из upstash.com |
| `UPSTASH_REDIS_REST_TOKEN` | Токен из upstash.com |

## Установка

1. Скачай репозиторий и импортируй в [Vercel](https://vercel.com)
2. Добавь переменные окружения выше
3. Зарегистрируй Telegram webhook:
```
GET https://api.telegram.org/botТОКЕН/setWebhook?url=https://проект.vercel.app/tg-update
```
4. Получи Twitch access token и подпишись на `stream.online` через Twitch EventSub API

## Использование

Напиши боту `/start` — появятся кнопки управления:

- **▶ Старт** — включить уведомления
- **⏹ Стоп** — выключить уведомления
- **📋 Сообщение** — показать текущее сообщение

Чтобы задать своё сообщение для стрима — просто напиши его боту.
