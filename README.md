# Discord Audio Bridge

A WebRTC to Discord audio bridge that allows you to stream audio from a web interface directly to a Discord voice channel.

## Description

This application creates a bridge between WebRTC audio streams and Discord voice channels. It enables:

- Real-time audio forwarding from a web browser to Discord
- Seamless Opus audio codec compatibility
- Simple web interface for establishing connections

## Features

- WebRTC audio capture from browser
- Discord voice channel integration with DAVE (E2EE) protocol support
- Automatic Opus codec handling
- Web-based control interface
- YouTube audio mixer with search and playlist support

## Requirements

- Go 1.26+
- Discord Bot with proper permissions (Connect, Speak)
- STUN server access (defaults to Google's public STUN server)

## Environment Variables

Copy the example environment file and fill in your values:

```bash
cp .env.example .env
```

| Variable | Description | Required |
|----------|-------------|----------|
| `DISCORD_BOT_TOKEN` | Your Discord bot token | Yes |
| `DISCORD_GUILD_ID` | Discord server ID | Yes |
| `DISCORD_CHANNEL_ID` | Voice channel ID | Yes |
| `YOUTUBE_API_KEY` | YouTube Data API key | No |

## Download

Pre-built binaries are available on the [Releases](https://github.com/CyTechNomad/discord-audio-bridge/releases) page.

Each release includes:
- Binary for your platform
- `.env` file (edit with your Discord credentials)
- `static/` folder (web interface)
- Launcher script (`start.sh` or `start.bat`)

## Installation

### From Release (Recommended)

1. Download the archive for your platform from [Releases](https://github.com/CyTechNomad/discord-audio-bridge/releases)
2. Extract the archive
3. Edit `.env` with your Discord bot token and channel IDs
4. Run `start.sh` (Linux/macOS) or `start.bat` (Windows)

### From Source

1. Clone the repository:

   ```bash
   git clone https://github.com/CyTechNomad/discord-audio-bridge.git
   cd discord-audio-bridge
   ```

2. Install dependencies:

   ```bash
   go mod download
   ```

3. Build the application:
   ```bash
   go build
   ```

## Usage

1. Start the server:

   ```bash
   ./discord-audio-bridge
   ```

2. Open your web browser and navigate to:

   ```
   http://localhost:8080
   ```

   Or for the YouTube mixer:

   ```
   http://localhost:8080/mixer.html
   ```

3. Grant permissions to capture tab audio in your browser and begin streaming to Discord

## Development

The server runs on port 8080 by default and provides:

- Static file serving from the `./static` directory
- WebRTC signaling via WebSocket at `/rtc`
- YouTube API key endpoint at `/api/youtube-key` (if configured)

## Contributing

This project uses [just](https://github.com/casey/just) as a command runner. Available commands:

```bash
just format   # Format all code (Go, HTML, YAML)
just build    # Build the application
just run      # Run the application
just deps     # Update dependencies
just clean    # Clean build artifacts
```

To format code before committing:

```bash
just format
```

## Acknowledgements

- [discordgo](https://github.com/bwmarrin/discordgo) - Discord API wrapper
- [yeongaori/discordgo](https://github.com/yeongaori/discordgo) - DAVE (E2EE) protocol support
- [Pion WebRTC](https://github.com/pion/webrtc) - WebRTC implementation
- [Gorilla WebSocket](https://github.com/gorilla/websocket) - WebSocket implementation
