// screenshot.go
package screenshot

import (
    "fmt"
    "image/png"
    "os"
    "path/filepath"

    "github.com/kbinani/screenshot"
)

// CaptureAndSave captures all active displays (the full virtual screen) and saves it as a PNG at the given path.
// It ensures the parent directory exists before saving.
func CaptureAndSave(path string) error {
    // Ensure parent directory exists
    dir := filepath.Dir(path)
    if err := os.MkdirAll(dir, 0o755); err != nil {
        return fmt.Errorf("failed to create directory %q: %w", dir, err)
    }

    // Compute full bounds across all displays
    bounds := screenshot.GetDisplayBounds(0)
    n := screenshot.NumActiveDisplays()
    for i := 1; i < n; i++ {
        b := screenshot.GetDisplayBounds(i)
        bounds = bounds.Union(b)
    }

    // Capture the image
    img, err := screenshot.CaptureRect(bounds)
    if err != nil {
        return fmt.Errorf("capture failed: %w", err)
    }

    // Create the output file
    file, err := os.Create(path)
    if err != nil {
        return fmt.Errorf("create file %q failed: %w", path, err)
    }
    defer file.Close()

    // Encode and write PNG
    if err := png.Encode(file, img); err != nil {
        return fmt.Errorf("encode PNG failed: %w", err)
    }

    return nil
}
