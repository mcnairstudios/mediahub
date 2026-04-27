# pkg/config — Configuration

## Purpose
Loads application configuration from environment variables with sensible defaults.

## Responsibilities
- Define the `Config` struct with all application-wide settings
- Load from `MEDIAHUB_*` environment variables
- Apply defaults for unset values

## Does NOT
- Depend on any other mediahub package — this is a leaf dependency
- Manage per-user or per-source settings — those are in pkg/store SettingsStore
