# Extension contract 1

The `contract` member of the manifest selects this contract. Package version and SDK module version are independent. Unsupported contracts are refused before native code starts.

## Manifest and signing

`Manifest` is the authoritative schema. It declares identity, publisher, operating-system backend paths, permissions, actions, frontend views, commands and the SHA-256 inventory of every payload file. An action has a name, title, description, input and output JSON schemas, permissions, replay policy and an optional effect level. A view has an ID, title, glyph and page, and may be a pane or a following window. A command refers to either a declared view or a declared action.

The signature is Ed25519 over UTF-8 JSON produced by Go's `encoding/json.Marshal` of `Manifest` with `Signature` set to the empty string. This is a specified Go serialization contract, not a language-independent canonical JSON standard. The signature includes the publisher public key and the complete inventory. Unknown manifest fields are rejected. Payload filenames are case-insensitively unique and portable across the supported desktop operating systems.

The installer pins the publisher key per extension ID. Consent binds both the fingerprint and exact package signature. A version already installed cannot be replaced by different content. Store registry changes are locked and replaced atomically. Removing an extension removes its package versions and private extension data.

An inventory has at most 4,096 files and 256 MiB total payload; an individual file is limited to 128 MiB. The manifest is limited to 1 MiB. Packages contain their manifest at the archive root.

## Backend transport

The backend is an MCP server over stdio using the public MCP Go SDK. Standard output is reserved for protocol messages. Diagnostics belong on standard error. `tools/list` must match the signed action names, schemas and metadata. Each tool's `_meta["com.parkscomputing.dinah/action"]` contains its `Action` declaration.

Dinah supplies `_meta["com.parkscomputing.dinah/host"]` on each `tools/call`. Its `Scope` contains a loopback callback URL, a secret bearer token and the authorized workspace snapshot. The SDK sends POST JSON `{tool,input}` to that URL with `Authorization: Bearer <token>`. Responses contain `{result}` or `{error}`. Tokens are valid only during their originating action. Extensions must not log or persist them.

This callback transport is a Dinah protocol. It does not assume MCP permits a server to issue a reverse `tools/call`. The callback server rejects browser Origin headers. Redirects are disabled by the SDK client.

Process actions and workspace writes use `replay: "once"`. A failed or interrupted action is never automatically replayed. Read-only actions with no process permission may use `pure`. A `process.run:<program>` action with no declared effect conservatively requires agent consent, including commands intended to read status.

## Effect levels

An action's optional `effect` member says what the action does to the person's work, so that Dinah can ask about the actions that matter and let the rest run. The four levels are these.

| Effect | Meaning | Examples |
| --- | --- | --- |
| `read` | Reads state and changes nothing. | status, log, diff, branch listing |
| `local` | Changes local state in a way the person can undo from inside the tool. | stage, unstage, switch branch, stash, commit |
| `history` | Rewrites or discards history or work that cannot be recovered from inside the tool. | discard, reset, force operations, delete branch |
| `network` | Reaches a remote or any network. | fetch, pull, push |

Dinah asks a chat's person for consent before an agent runs an action whose effect is `history` or `network`, and before an action with no declared effect that has a process, write or terminal permission. Show Me First, when the person turns it on for the extension, reviews the external commands of those same actions and lets `read` and `local` commands run without a review. The level is part of the signed manifest, and the backend's `tools/list` metadata must match it. The level does not change what an action may call; permissions still bound every host service, and a frame's own confirmations for destructive operations remain the extension's responsibility.

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
| `dinah.on(event,handler)` | Subscribes and returns an unsubscribe function. Events include `workspace`, `appearance`, `state`, `command` and `shared`. |
| `dinah.commands(list)` | Replaces the view's current command descriptors. Each descriptor has an ID, label and optional glyph, disabled, danger and shortcut fields. The ID `pin-workspace` is reserved for the host's pin command. |
| `dinah.navigate(state)` | Writes a string of up to 8,192 characters into the owning window's addressable navigation state. |
| `dinah.hostCommand(name,payload)` | Invokes an allowlisted host command, with an optional JSON payload. Contract 1 exposes `focus-menubar`, `keyboard-shortcuts`, `toggle-fullscreen`, `open-view/<view-id>` and `open-diff`. |
| `dinah.shared.get()` | Returns a copy of the extension's shared map for the frame's current workspace. |
| `dinah.shared.set(key,value)` | Writes one key of the shared map and resolves when the host has delivered it. A value of `null` or `undefined` removes the key. |

### Shared state

Frames of one extension that show the same workspace share one map of JSON values, which the host keeps in memory while any frame of the extension lives. The map is not persisted and is not visible to other extensions. A key matches `^[a-z0-9][-a-z0-9_.]{0,63}$`; a value's JSON is at most 64 KiB, and a map holds at most 64 keys. A newly opened frame receives the current map before `dinah.ready` resolves, so `dinah.shared.get()` is complete at that point. Each change made by another frame arrives as a `shared` event `{key,value,from}`, where `from` is the view ID of the frame that wrote it and `value` is `null` for a removal. The writer does not receive its own event. When a following view moves to another workspace, the frame receives that workspace's map as `shared` events with an empty `from`, one per key.

### Views, windows and the workspace

A view declared `pane: true` is docked and follows the selected workspace. A view declared `follows: true` opens as a window that also follows the selected workspace, and the host adds a checkable "Pin to This Workspace" command to the window's own commands; while pinned, the window keeps the workspace it had when pinned, and unpinning lets it follow again. The window's title detail shows the workspace's name while the view follows. A view with neither member opens a window that takes the selected workspace at opening and keeps it. Every change of a frame's workspace, including a pin or unpin, arrives as a `workspace` event whose snapshot is the workspace; a following view's snapshot also has a `pinned` boolean.

`dinah.hostCommand('open-view/<view-id>')` opens one of the extension's own declared views with the calling frame's current addressable state. When a window of that view is already open in the same workspace, the host raises it and delivers the new state through its `state` event instead of opening another. Passing `{reuse: false}` as the payload opens a further window. The result is `{window}`, the host's key for the window.

### Diffs in the Editor

`dinah.hostCommand('open-diff', spec)` opens a diff in Dinah's Editor. `DiffSpec` in the Go package is the authoritative shape: `{key, title, detail, path, language, before, after, mode, readOnly, actions}`, where `mode` is `inline` or `split` and each action is `{id, label, glyph, scope, primary, danger}` with `scope` one of `file`, `hunk` or `line`. The host refuses a specification that fails `DiffSpec.Validate`: a key and title of at most 200 characters, a detail of at most 500, a path of at most 1,024, a side of at most 2 MiB, at most 20 actions, and action IDs matching `^[-a-z0-9]{1,64}$`. When the person takes an action in the diff, the frame receives a `command` event `{id: 'diff/<action id>', diff: key, where}`, where `where` describes the file, hunk or line range the Editor supplied. A host whose Editor has no diff view fails with "The Editor's diff view is not available".

The frame uses `sandbox="allow-scripts allow-forms"`, retains an opaque origin, and receives a response CSP with `form-action 'none'`. Forms can perform local validation and submit handlers; browser form requests are blocked. External network connections are unavailable unless the manifest declares `network`, which permits HTTPS connections. The frame cannot use the native Go bridge or inspect the parent DOM. Local popovers remain inside its bounds.

Appearance includes the resolved light/dark mode, theme identity, direction and approved CSS tokens. PUDL font assets are served without credentials and with CORS headers suitable for opaque frames. The frame should preserve text selection and use native focus controls.

F10 forwards the explicit `focus-menubar` host command. The SDK records its focused child element under a fresh excursion token. The host restores focus to the frame and echoes that token through `focus-return`; the SDK restores the child element or a visible `[data-extension-focus]`, main or body fallback. Extension commands follow focus-return over the same ordered channel. No key events or child DOM references cross the boundary.

A frame navigation, replacement, disable or removal invalidates its channel and cancels pending actions. A replacement must establish a fresh channel. The initial global handshake accepts a transferred port only from the expected parent window; the opaque origin string `null` is never used as an identity.

## Process byte preservation

JSON strings display invalid UTF-8 bytes as replacement characters. Process inputs and results therefore carry optional `argsBase64`, `stdinBase64`, `stdoutBase64` and `stderrBase64` fields when required. Base64 uses the standard padded encoding. The encoded field is authoritative when present, and the SDK handles the conversion transparently. Terminal requests use the same optional `argsBase64` array. Command logs retain encoded arguments and mark commands that used stdin; those commands cannot be replayed by the current Terminal handoff. This preserves Unix filename and patch bytes across the host protocol.

A host moving focus into its own native chrome may first send `focus-prepare` with a fresh 16 to 128 character token over the authenticated port. The SDK records its last child focus target and invalidates an older excursion. A later matching `focus-return` restores that control or the documented usable fallback. Hosts send preparation before return on the same ordered port, verify the live frame actually regained focus before returning, and invalidate excursions when a frame is replaced. Unknown or stale tokens never move focus.
