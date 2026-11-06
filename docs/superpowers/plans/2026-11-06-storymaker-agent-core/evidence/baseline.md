# Test baseline at 5e9e48e92664ef7e0c9c97dc3e6ff49f18f8cd31

A batch passes when its failures are a subset of this file.

## ./internal/agent
- TestFollowUpAndWaitForEventNeedARuleWhenUnattended
- TestUnattendedFetchCopyingIsolatedTextNeedsARule

## ./internal/server -run Storymaker
none

## ./internal/storymaker/...
none

## ./internal/config
none

## ./internal/prompts
none

## ./internal/security
none

## ./internal/audit
- TestCredentialedHTTPClientsBindRedirectsToOrigin
- TestSubprocessLaunchesDoNotInheritRawProcessEnvironment
- TestUpdateConfigFallbackOnlyTrustedTransactionBackups

## ./ui -run (?i)(storymaker|config|i18n|registration|TestShippedWebAssetsLoadNothingExternal)
none
