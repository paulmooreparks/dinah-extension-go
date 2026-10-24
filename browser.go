package extension

import _ "embed"

//go:embed web/dinah-extension.js
var BrowserBridge []byte
