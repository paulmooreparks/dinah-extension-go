# Changelog

## 0.2.0-rc.1 (not yet tagged)

Every change in this release is additive within contract 1. A package that uses none of the new manifest members signs and installs as it did with 0.1.0. A package that uses one installs only in a host built with 0.2.0 or later; older hosts refuse it at installation because they reject manifest fields they do not know. `Manifest.MinimumSDK` returns the release a manifest needs, and the `sign` and `verify` commands print it.

- A view's optional `opens` member lists the media types and file extensions it opens, so that a link to such a file in Dinah opens in the view with the file in its navigation state (`OpenState`, `ParseOpenState` and `dinah.openedFile`). Paul approved this change to the contract on 28 October 2026.
- `Installation.EnabledAt` records when an extension was enabled, and an update keeps it, so the host can let the extension enabled first open a file type two extensions declare.
- An action's optional `effect` member declares whether it reads, changes local state, rewrites history or reaches a network, and `EffectAsks` gives the consent rule.
- A view's optional `follows` member makes a window follow the selected workspace, with a pin.
- The browser bridge gains shared state (`dinah.shared`), a payload argument on `dinah.hostCommand`, and the `open-diff` host command, whose specification is `DiffSpec`.
- A host may prepare a frame for a focus excursion into its own chrome with `focus-prepare`, and the frame restores the control that had focus when the matching `focus-return` arrives.
- A package removal that fails stays registered, so that it can be retried.
- The SDK is licensed under Apache 2.0.

## 0.1.0

The first release: signed packages, the MCP backend transport, host services and the browser bridge.
