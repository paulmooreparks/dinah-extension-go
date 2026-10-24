# Dinah Extension Go SDK

This module is the public contract for native Dinah extensions. An extension backend implements standard MCP over stdio. Its frontend runs in an opaque-origin frame and calls its declared actions through the embedded browser bridge.

The SDK compiles without Dinah's source. It requires Go 1.25 or later.

## Build an extension

Declare an `extension.Manifest` and a map of `extension.Handler` functions, then call `extension.Serve`. A handler receives an invocation-scoped `*extension.Host`. Use `Host.Run` to execute a declared program and `Host.Call` for other host services. The host records commands and enforces the action's declared permissions on callbacks.

Each package contains `dinah-extension.json`, a native executable for each supported operating system, frontend pages, and their assets. Relative paths use forward slashes. Packages contain regular files only. Symlinks, case-colliding names, Windows device names, traversal and unsigned payload files are rejected.

```powershell
go run github.com/paulmooreparks/dinah-extension-go/cmd/dinah-extension keygen C:/private/publisher.private-key
go run github.com/paulmooreparks/dinah-extension-go/cmd/dinah-extension sign ./dist/extension C:/private/publisher.private-key
go run github.com/paulmooreparks/dinah-extension-go/cmd/dinah-extension verify ./dist/extension
go run github.com/paulmooreparks/dinah-extension-go/cmd/dinah-extension zip ./dist/extension ./extension.zip
```

Keep the private key outside source repositories and packages. Losing it prevents normal updates to installations that have pinned its public key. Dinah presents the publisher fingerprint and permissions at installation. The fingerprint authenticates later updates; it does not establish the publisher's real-world identity.

## Native trust

Installing an extension never executes its backend. Running an installed extension executes native code with the user's operating-system authority. Host permissions constrain host API requests; they do not sandbox arbitrary native code. Only install publishers you trust.

The frame receives no native bridge, host cookies or persistent browser storage. It loads the host's pinned PUDL assets and receives appearance, workspace and navigation state through a dedicated message channel. Native callback authority expires when its action ends or is cancelled.

Read [CONTRACT.md](CONTRACT.md) for the versioned wire contract and host services. The [Git extension](https://github.com/paulmooreparks/dinah-git) provides a separate application built against this SDK.

## Verification

Run `go test ./...` and `go vet ./...`. Package tests cover signed inventories, pinned publisher updates, installation consent and unsafe archive paths. Dinah's integration tests exercise the native MCP process, scope revocation and sandboxed frame lifecycle.
