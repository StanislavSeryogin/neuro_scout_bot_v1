# Neuro Scout Bot V1

A Go-based project for the Neuro Scout Bot.

## Getting Started

### Prerequisites

- Go 1.16+
- PostgreSQL database

### Installation

1. Clone the repository
```bash
git clone https://github.com/yourusername/neuro_scout_bot_v1.git
cd neuro_scout_bot_v1
```

2. Configure the application
```bash
# Copy the template configuration file
cp config.template.hcl config.local.hcl

# Edit the configuration file with your own settings
nano config.local.hcl
```

3. Build the project
```bash
go build
```

4. Run the application
```bash
./neuro_scout_bot_v1
```

### Configuration

Edit `config.local.hcl` with your personal settings:

- `telegram_bot_token` - Your Telegram Bot token from BotFather
- `telegram_channel_id` - Your Telegram channel ID 
- `database_dsn` - Database connection string
- `fetch_interval` - How often to fetch new content
- `notification_interval` - How often to send notifications
- `openai_key` - (Optional) OpenAI API key for summarization
- `openai_model` - OpenAI model to use
- `openai_prompt` - Prompt for generating summaries

## Project Structure

- `main.go` - Main application entry point
- `pkg/` - Package directory for shared code
- `internal/` - Private application code 