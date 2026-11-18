# Notes for coding agents

- Keep the core in `internal/` free of UI code; the command line (`cmd/lieferling`) and the Wails app (`main.go`, `service.go`) are thin layers over `internal/store`.
- Run `go test ./internal/... ./cmd/...` after changing the core.
- The app's UI is German; the command line and code comments are English.
- New platforms get their own `internal/install/install_<os>.go`; `pick` already chooses assets for Windows (`.exe`, `.msi`) and Linux (`.AppImage`).

## Frontend

- Use the `modern-web-guidance` skill before writing HTML, CSS or client-side JS, and the `impeccable` skill for design work. `PRODUCT.md` describes the product.
- **Browser support:** the app runs in the system web view (WKWebView on macOS, WebKitGTK on Linux, WebView2 on Windows). Target Safari 17.5 and later: Baseline 2024 features are fine without fallbacks (`light-dark()`, popover, `:user-invalid`); newer ones need a fallback of at most 20 lines and no dependency. No polyfills.
- `frontend/bindings` is generated (`wails3 generate bindings -ts -i`); do not edit it.
