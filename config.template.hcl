telegram_bot_token = "YOUR_TELEGRAM_BOT_TOKEN"
telegram_channel_id = 0  # Your Telegram channel ID
fetch_interval = "1h"  # Interval for fetching new content
notification_interval = "30m"  # Interval for sending notifications
database_dsn = "postgres://username:password@localhost:5432/dbname?sslmode=disable"

# OpenAI configuration (optional)
# openai_key = "YOUR_OPENAI_API_KEY"  # Uncomment and set to enable summarization
openai_model = "gpt-3.5-turbo" 
openai_prompt = "Create a concise summary of the following article in English. Focus on the main points, key insights, and conclusions. The summary should be 3-5 sentences long." 