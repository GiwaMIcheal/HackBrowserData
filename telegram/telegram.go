// telegram.go
package telegram

import (
    "bytes"
    "fmt"
    "io"
    "mime/multipart"
    "net/http"
    "os"
    "path/filepath"
    "time"

    "github.com/yourusername/yourmodule/config"
)

const telegramAPI = "https://api.telegram.org"

// httpClient is a shared client with a timeout for Telegram requests.
var httpClient = &http.Client{
    Timeout: 30 * time.Second,
}

// SendText sends a text message via Telegram Bot API.
func SendText(text string) error {
    url := fmt.Sprintf("%s/bot%s/sendMessage", telegramAPI, config.TelegramBotToken)
    resp, err := http.PostForm(url, map[string][]string{
        "chat_id": {config.TelegramChatID},
        "text":    {text},
    })
    if err != nil {
        return fmt.Errorf("SendText: HTTP request failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("SendText: non-OK HTTP status %d: %s", resp.StatusCode, string(body))
    }
    return nil
}

// SendPhoto sends a photo file (e.g. PNG, JPEG) via Telegram Bot API.
func SendPhoto(photoPath string) error {
    return sendFile(photoPath, "photo")
}

// SendDocument sends a generic file (e.g. ZIP) via Telegram Bot API.
func SendDocument(docPath string) error {
    return sendFile(docPath, "document")
}

// sendFile sends a file under the given field name ("photo" or "document").
func sendFile(path string, fieldName string) error {
    // Open the file
    file, err := os.Open(path)
    if err != nil {
        return fmt.Errorf("sendFile: could not open file %q: %w", path, err)
    }
    defer file.Close()

    // Get file info for the filename
    fi, err := file.Stat()
    if err != nil {
        return fmt.Errorf("sendFile: could not stat file %q: %w", path, err)
    }

    // Prepare multipart form
    var buf bytes.Buffer
    writer := multipart.NewWriter(&buf)

    // chat_id field
    if err := writer.WriteField("chat_id", config.TelegramChatID); err != nil {
        writer.Close()
        return fmt.Errorf("sendFile: failed to write chat_id field: %w", err)
    }

    // Optionally, could add caption field here if desired:
    // writer.WriteField("caption", "Your caption here")

    // Create file part
    part, err := writer.CreateFormFile(fieldName, filepath.Base(fi.Name()))
    if err != nil {
        writer.Close()
        return fmt.Errorf("sendFile: CreateFormFile failed: %w", err)
    }
    if _, err := io.Copy(part, file); err != nil {
        writer.Close()
        return fmt.Errorf("sendFile: copying file content failed: %w", err)
    }

    // Close writer to finalize the form
    if err := writer.Close(); err != nil {
        return fmt.Errorf("sendFile: closing multipart writer failed: %w", err)
    }

    // Build request URL, e.g. /sendPhoto or /sendDocument
    url := fmt.Sprintf("%s/bot%s/send%s", telegramAPI, config.TelegramBotToken, capitalize(fieldName))
    req, err := http.NewRequest("POST", url, &buf)
    if err != nil {
        return fmt.Errorf("sendFile: creating HTTP request failed: %w", err)
    }
    req.Header.Set("Content-Type", writer.FormDataContentType())

    // Perform the request
    resp, err := httpClient.Do(req)
    if err != nil {
        return fmt.Errorf("sendFile: HTTP request failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        respBody, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("sendFile: non-OK HTTP status %d: %s", resp.StatusCode, string(respBody))
    }
    return nil
}

// capitalize returns the input string with first letter uppercase, rest unchanged.
// E.g., "photo" -> "Photo", so sendPhoto uses sendPhoto -> sendPhoto endpoint "sendPhoto".
func capitalize(s string) string {
    if s == "" {
        return s
    }
    b := []rune(s)
    if b[0] >= 'a' && b[0] <= 'z' {
        b[0] = b[0] - ('a' - 'A')
    }
    return string(b)
}
