# Discord Rich Presence Setup

Maggus supports Discord Rich Presence to show "Running Maggus" in your Discord profile while `maggus work` is active. This document explains how the Discord Application was set up (for future maintainers).

## Discord Application

The Rich Presence integration requires a Discord Application registered in the Developer Portal. This provides an Application ID that the local IPC protocol uses to identify the activity.

### Creating the Application

1. Go to https://discord.com/developers/applications
2. Click **New Application**
3. Name it **Maggus**
4. Note the **Application ID** from the General Information page — this is the value used in `internal/discord/discord.go` as `ApplicationID`

### Uploading the Application Icon

1. In the application's **General Information** page, click the icon placeholder
2. Upload `docs/avatar.png` (or `src/winres/icon.png`) as the application icon
3. This icon appears in Discord profiles when Maggus Rich Presence is active

### Uploading Rich Presence Assets

1. Navigate to **Rich Presence** > **Art Assets** in the application settings
2. Click **Add Image(s)**
3. Upload `docs/avatar.png` (or `src/winres/icon.png`)
4. Set the asset key to exactly: `maggus_logo`
5. Save changes — assets may take a few minutes to propagate

## Integration Details

- The Application ID is hardcoded in `src/internal/discord/discord.go` as a constant (it is a public value, not a secret)
- The asset key `maggus_logo` is referenced when setting the large image in Rich Presence updates
- Discord Rich Presence uses local IPC only — no network requests, no bot token, no OAuth
- Platform-specific IPC: Unix socket (`/tmp/discord-ipc-0`) on Linux/macOS, named pipe (`\\.\pipe\discord-ipc-0`) on Windows

## User Configuration

Users enable Rich Presence by adding to `.maggus/config.yml`:

```yaml
discord_presence: true
```

When disabled or absent, no Discord connection is attempted.
