# Extension contract 1

The `contract` member of the manifest selects this contract. Package version and SDK module version are independent. Unsupported contracts are refused before native code starts.

## Manifest and signing

`Manifest` is the authoritative schema. It declares identity, publisher, operating-system backend paths, permissions, actions, frontend views, commands and the SHA-256 inventory of every payload file. An action has a name, title, description, input and output JSON schemas, permissions, replay policy and an optional effect level. A view has an ID, title, glyph and page, and may be a pane or a following window; a window that is neither may declare the kinds of file it opens (see Views that open files). A command refers to either a declared view or a declared action.

The signature is Ed25519 over UTF-8 JSON produced by Go's `encoding/json.Marshal` of `Manifest` with `Signature` set to the empty string. This is a specified Go serialization contract, not a language-independent canonical JSON standard. The signature includes the publisher public key and the complete inventory. Unknown manifest fields are rejected. Payload filenames are case-insensitively unique and portable across the supported desktop operating systems.

The installer pins the publisher key per extension ID. Consent binds both the fingerprint and exact package signature. A version already installed cannot be replaced by different content. Store registry changes are locked and replaced atomically. Removing an extension removes its package versions and private extension data.

An inventory has at most 4,096 files and 256 MiB total payload; an individual file is limited to 128 MiB. The manifest is limited to 1 MiB. Packages contain their manifest at the archive root.

### Members added within contract 1

Contract 1 grows only by optional members. A manifest that leaves them out is unchanged: it serializes, signs and validates exactly as it did before they existed, so existing packages keep their signatures and install in every host. Because a host rejects manifest fields it does not know, a package that uses an added member installs only in a host built with the SDK release that added it or a later one. An older host refuses such a package at installation with an unknown-field error, before anything is stored or run, so it never runs a package with part of its manifest ignored.

| Member | Added in SDK |
| --- | --- |
| Action `effect` | 0.2.0 |
| View `follows` | 0.2.0 |
| View `opens` | 0.2.0 |

`Manifest.MinimumSDK` returns the earliest release whose hosts accept a manifest, and `dinah-extension sign` and `verify` print it when it is later than 0.1.0. A publisher who needs a package to install in older hosts leaves the newer members out.

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
| `dinah.openedFile(state)` | Returns the file in a state Dinah gave the view to open a file, `{path, mediaType, resource, line}`, or `null` for any other state. |
| `dinah.shared.get()` | Returns a copy of the extension's shared map for the frame's current workspace. |
| `dinah.shared.set(key,value)` | Writes one key of the shared map and resolves when the host has delivered it. A value of `null` or `undefined` removes the key. |

### Shared state

Frames of one extension that show the same workspace share one map of JSON values, which the host keeps in memory while any frame of the extension lives. The map is not persisted and is not visible to other extensions. A key matches `^[a-z0-9][-a-z0-9_.]{0,63}$`; a value's JSON is at most 64 KiB, and a map holds at most 64 keys. A newly opened frame receives the current map before `dinah.ready` resolves, so `dinah.shared.get()` is complete at that point. Each change made by another frame arrives as a `shared` event `{key,value,from}`, where `from` is the view ID of the frame that wrote it and `value` is `null` for a removal. The writer does not receive its own event. When a following view moves to another workspace, the frame receives that workspace's map as `shared` events with an empty `from`, one per key.

### Views, windows and the workspace

A view declared `pane: true` is docked and follows the selected workspace. A view declared `follows: true` opens as a window that also follows the selected workspace, and the host adds a checkable "Pin to This Workspace" command to the window's own commands; while pinned, the window keeps the workspace it had when pinned, and unpinning lets it follow again. The window's title detail shows the workspace's name while the view follows. A view with neither member opens a window that takes the selected workspace at opening and keeps it. Every change of a frame's workspace, including a pin or unpin, arrives as a `workspace` event whose snapshot is the workspace; a following view's snapshot also has a `pinned` boolean.

`dinah.hostCommand('open-view/<view-id>')` opens one of the extension's own declared views with the calling frame's current addressable state. When a window of that view is already open in the same workspace, the host raises it and delivers the new state through its `state` event instead of opening another. Passing `{reuse: false}` as the payload opens a further window. The result is `{window}`, the host's key for the window.

### Views that open files

A window view may declare the kinds of file it opens, so that a link to such a file in Dinah opens in the view. The optional `opens` member is a list of file types, each an IANA media type and the file extensions that have it:

```json
{"id": "patch", "title": "Patch", "glyph": "document", "page": "patch.html",
 "opens": [{"mediaType": "text/x-diff", "extensions": [".diff", ".patch"]}]}
```

`FileType` and `View.Opens` in the Go package are the authoritative shape, and `Manifest.Validate` enforces these rules:

- A view that declares `opens` is a window that is neither `pane` nor `follows`, because a file belongs to one workspace and such a window keeps the workspace it opened with.
- The manifest has the `workspace.read` permission, which the view needs to read the file.
- A media type is a registered top-level type (`application`, `audio`, `font`, `image`, `model`, `text` or `video`), a slash and a subtype of the characters RFC 6838 allows, in lower case, with no parameters, and at most 127 characters long.
- `text/plain` and `application/octet-stream` are refused, because they describe any file rather than a kind of file.
- A file type lists between 1 and 16 extensions. An extension is a dot followed by one segment of at most 31 lower-case letters, digits, underscores, plus signs and hyphens.
- A view declares at most 8 file types and a manifest at most 32. A media type, and likewise an extension, appears at most once in a manifest.

Wildcards such as `image/*` are not accepted. The host decides by a file's extension, so a wildcard would not say which files it covers, and it would quietly take in types that a later host opens itself or that the publisher never tried. A view that opens several types lists each of them.

`opens` is signed with the rest of the manifest. It was added in SDK 0.2.0, and an older host refuses a package that uses it, as Members added within contract 1 describes.

A file matches a declared type when the host already knows the file's extension as that media type, or when the host has no media type of its own for the extension and the type lists it. A declaration never changes the type of an extension the host already knows. A declaration is a request, and the host decides precedence: a kind of file the host opens in its own views stays there, and when two extensions declare the same type or extension, the one enabled first opens it. A view therefore cannot rely on receiving every file it declares. The host opens a declared file in the view only when the file is inside a source of one of the person's workspaces.

When the person opens such a file, the host opens the view in the file's workspace with this navigation state, the JSON text of `OpenState`:

```json
{"open": {"path": "C:\\work\\project\\fix.diff", "mediaType": "text/x-diff",
          "resource": {"workspaceId": "...", "sourceId": "...", "path": "fix.diff"},
          "line": 12}}
```

`path` is the file's absolute path, which `host.read` accepts within its 2 MiB limit. `mediaType` is the declared type the file matched. `resource` is the file's workspace, the workspace source it is in and its path within that source with forward slashes. `line` is present when the link asked for a line. A newly opened window receives the state from `dinah.ready`; when a window of the view is already open in that workspace, the host raises it and delivers the state through its `state` event, as `open-view` does. `dinah.openedFile(state)` in the browser bridge and `ParseOpenState` in the Go package read the state and return nothing for any other state, so a view may keep states of its own. The state is the window's addressable state like any other: a view that later calls `dinah.navigate` replaces it, and a window restored after a restart receives whatever the view wrote last.

### Diffs in the Editor

`dinah.hostCommand('open-diff', spec)` opens a diff in Dinah's Editor. `DiffSpec` in the Go package is the authoritative shape: `{key, title, detail, path, language, before, after, mode, readOnly, actions}`, where `mode` is `inline` or `split` and each action is `{id, label, glyph, scope, primary, danger}` with `scope` one of `file`, `hunk` or `line`. The host refuses a specification that fails `DiffSpec.Validate`: a key and title of at most 200 characters, a detail of at most 500, a path of at most 1,024, a side of at most 2 MiB, at most 20 actions, and action IDs matching `^[-a-z0-9]{1,64}$`. When the person takes an action in the diff, the frame receives a `command` event `{id: 'diff/<action id>', diff: key, where}`, where `where` describes the file, hunk or line range the Editor supplied. A host whose Editor has no diff view fails with "The Editor's diff view is not available".

The frame uses `sandbox="allow-scripts allow-forms"`, retains an opaque origin, and receives a response CSP with `form-action 'none'`. Forms can perform local validation and submit handlers; browser form requests are blocked. External network connections are unavailable unless the manifest declares `network`, which permits HTTPS connections. The frame cannot use the native Go bridge or inspect the parent DOM. Local popovers remain inside its bounds.

Appearance includes the resolved light/dark mode, theme identity, direction and approved CSS tokens. PUDL font assets are served without credentials and with CORS headers suitable for opaque frames. The frame should preserve text selection and use native focus controls.

F10 forwards the explicit `focus-menubar` host command. The SDK records its focused child element under a fresh excursion token. The host restores focus to the frame and echoes that token through `focus-return`; the SDK restores the child element or a visible `[data-extension-focus]`, main or body fallback. Extension commands follow focus-return over the same ordered channel. No key events or child DOM references cross the boundary.

A frame navigation, replacement, disable or removal invalidates its channel and cancels pending actions. A replacement must establish a fresh channel. The initial global handshake accepts a transferred port only from the expected parent window; the opaque origin string `null` is never used as an identity.

## Process byte preservation

JSON strings display invalid UTF-8 bytes as replacement characters. Process inputs and results therefore carry optional `argsBase64`, `stdinBase64`, `stdoutBase64` and `stderrBase64` fields when required. Base64 uses the standard padded encoding. The encoded field is authoritative when present, and the SDK handles the conversion transparently. Terminal requests use the same optional `argsBase64` array. Command logs retain encoded arguments and mark commands that used stdin; those commands cannot be replayed by the current Terminal handoff. This preserves Unix filename and patch bytes across the host protocol.

A host moving focus into its own native chrome may first send `focus-prepare` with a fresh 16 to 128 character token over the authenticated port. The SDK records its last child focus target and invalidates an older excursion. A later matching `focus-return` restores that control or the documented usable fallback. Hosts send preparation before return on the same ordered port, verify the live frame actually regained focus before returning, and invalidate excursions when a frame is replaced. Unknown or stale tokens never move focus.
