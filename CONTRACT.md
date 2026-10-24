# Extension contract 1

The `contract` member of the manifest selects this contract. Package version and SDK module version are independent. Unsupported contracts are refused before native code starts.

## Manifest and signing

`Manifest` is the authoritative schema. It declares identity, publisher, operating-system backend paths, permissions, actions, frontend views, commands and the SHA-256 inventory of every payload file. An action has a name, title, description, input and output JSON schemas, permissions and replay policy. A command names either a declared view or a declared action.

The signature is Ed25519 over UTF-8 JSON produced by Go's `encoding/json.Marshal` of `Manifest` with `Signature` set to the empty string. This is a specified Go serialization contract, not a language-independent canonical JSON standard. The signature includes the publisher public key and the complete inventory. Unknown manifest fields are rejected. Payload filenames are case-insensitively unique and portable across the supported desktop operating systems.

The installer pins the publisher key per extension ID. Consent binds both the fingerprint and exact package signature. A version already installed cannot be replaced by different content. Store registry changes are locked and replaced atomically. Removing an extension removes its package versions and private extension data.

An inventory has at most 4,096 files and 256 MiB total payload; an individual file is limited to 128 MiB. The manifest is limited to 1 MiB. Packages contain their manifest at the archive root.

## Backend transport

The backend is an MCP server over stdio using the public MCP Go SDK. Standard output is reserved for protocol messages. Diagnostics belong on standard error. `tools/list` must match the signed action names, schemas and metadata. Each tool's `_meta["com.parkscomputing.dinah/action"]` contains its `Action` declaration.

Dinah supplies `_meta["com.parkscomputing.dinah/host"]` on each `tools/call`. Its `Scope` contains a loopback callback URL, a secret bearer token and the authorized workspace snapshot. The SDK sends POST JSON `{tool,input}` to that URL with `Authorization: Bearer <token>`. Responses contain `{result}` or `{error}`. Tokens are valid only during their originating action. Extensions must not log or persist them.

This callback transport is a Dinah protocol. It does not assume MCP permits a server to issue a reverse `tools/call`. The callback server rejects browser Origin headers. Redirects are disabled by the SDK client.

Process actions and workspace writes use `replay: "once"`. A failed or interrupted action is never automatically replayed. Read-only actions with no process permission may use `pure`. All `process.run:<program>` actions conservatively require agent consent, including commands intended to read status.

## Host services

Each callback is checked against the permissions of the current action, which must be a subset of the manifest's permissions. Workspaces contain a stable ID, display name, execution path and local sources. Relative file paths resolve from the workspace execution path.

| Service | Input and result | Permission |
| --- | --- | --- |
| `host.workspaces` | Returns the single authorized workspace snapshot. | `workspace.read` |
| `host.run` | Accepts `RunInput`; returns `RunResult`. | `process.run:<program>` |
| `host.resolve` | Accepts `{path,create}`; resolves an existing path or a new leaf under an existing parent. Returns `{path}`. | `workspace.read` |
| `host.read` | Accepts `FileInput`; returns text and SHA-256. | `workspace.read` |
| `host.write` | Accepts `FileInput`; existing files require `expectedSha256`. Returns the written text and SHA-256. | `workspace.write` |
| `host.commandlog` | Returns the extension's last 100 command entries within the current workspace. | `workspace.read` |
| `host.terminal` | Accepts `TerminalInput`; prepares an explicit argument array for the person to run. | `terminal.open` and the program permission |
| `host.notify` | Accepts `{title,body}`; posts a notification. | `notification.post` |
| `host.watch` | Accepts `{path}`; waits up to 20 seconds for a filesystem event and returns `{changed,path?}`. Watching is not recursive. | `workspace.read` |
| `host.decorate` | Reserved for host-supported file and editor decorations. Hosts without this implementation return an unavailable error. | `workspace.decorate` |

`RunInput` supplies a simple program name, literal argument array, working directory, optional stdin, explanation and timeout. The host invokes the program directly without shell concatenation. The directory must resolve inside the authorized workspace or a local source. Arguments have a 128 KiB combined limit; stdin is limited to 4 MiB. The timeout defaults to 30 seconds and is capped at 120 seconds. Stdout and stderr each retain at most 2 MiB and report truncation. Commands retain their arguments, explanation, exit code, duration and bounded output in the extension's command log.

The host's Show Me First preference can require review before each process launch. The person may edit the argument array, run it, prepare it in Terminal or cancel. Editing a machine-readable command may cause the extension to reject its result. A command with stdin cannot be handed to Terminal through this API because that would lose the input patch.

File operations use an open directory root to prevent symlink traversal outside allowed roots. They are limited to 2 MiB. The expected hash detects an external change observed before replacement; it is not a filesystem transaction with unrelated writers. Native processes can independently access other files using their operating-system authority.

## Browser bridge

Load `/extension-sdk/dinah-extension.js` before the extension's own scripts. Load PUDL assets from `/pudl/`. Do not load `pudl-theme.js` because the host owns appearance.

`dinah.ready` resolves to `{workspace,state}` after the dedicated channel and initial appearance are ready. `dinah.workspace` and `dinah.theme` hold the latest snapshots.

| API | Behavior |
| --- | --- |
| `dinah.call(action,input)` | Calls a declared action of this extension and resolves to its structured output. |
| `dinah.on(event,handler)` | Subscribes and returns an unsubscribe function. Events include `workspace`, `appearance`, `state` and `command`. |
| `dinah.commands(list)` | Replaces the view's current command descriptors. Each descriptor has an ID, label and optional glyph, disabled, danger and shortcut fields. |
| `dinah.navigate(state)` | Writes a string of up to 8,192 characters into the owning window's addressable navigation state. |
| `dinah.hostCommand(name)` | Invokes an allowlisted host command. Contract 1 exposes `focus-menubar` and `keyboard-shortcuts`. |

The frame uses `sandbox="allow-scripts allow-forms"`, retains an opaque origin, and receives a response CSP with `form-action 'none'`. Forms can perform local validation and submit handlers; browser form requests are blocked. External network connections are unavailable unless the manifest declares `network`, which permits HTTPS connections. The frame cannot use the native Go bridge or inspect the parent DOM. Local popovers remain inside its bounds.

Appearance includes the resolved light/dark mode, theme identity, direction and approved CSS tokens. PUDL font assets are served without credentials and with CORS headers suitable for opaque frames. The frame should preserve text selection and use native focus controls.

F10 forwards the explicit `focus-menubar` host command. The SDK records its focused child element under a fresh excursion token. The host restores focus to the frame and echoes that token through `focus-return`; the SDK restores the child element or a visible `[data-extension-focus]`, main or body fallback. Extension commands follow focus-return over the same ordered channel. No key events or child DOM references cross the boundary.

A frame navigation, replacement, disable or removal invalidates its channel and cancels pending actions. A replacement must establish a fresh channel. The initial global handshake accepts a transferred port only from the expected parent window; the opaque origin string `null` is never used as an identity.

## Process byte preservation

JSON strings display invalid UTF-8 bytes as replacement characters. Process inputs and results therefore carry optional `argsBase64`, `stdinBase64`, `stdoutBase64` and `stderrBase64` fields when required. Base64 uses the standard padded encoding. The encoded field is authoritative when present, and the SDK handles the conversion transparently. Terminal requests use the same optional `argsBase64` array. Command logs retain encoded arguments and mark commands that used stdin; those commands cannot be replayed by the current Terminal handoff. This preserves Unix filename and patch bytes across the host protocol.
