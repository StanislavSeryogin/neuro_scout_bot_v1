package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"neuro_scout_bot_v1/internal/bot"
	"neuro_scout_bot_v1/internal/botkit"
	"neuro_scout_bot_v1/internal/config"
	"neuro_scout_bot_v1/internal/fetcher"
	"neuro_scout_bot_v1/internal/notifier"
	"neuro_scout_bot_v1/internal/storage"
	"neuro_scout_bot_v1/internal/summary"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // PostgreSQL driver
)

func main() {
	botAPI, err := tgbotapi.NewBotAPI(config.Get().TelegramBotToken)
	if err != nil {
		log.Printf("[ERROR] Failed to create bot API: %v", err)
		return
	}

	botAPI.Debug = true
	_, err = botAPI.Request(tgbotapi.DeleteWebhookConfig{
		DropPendingUpdates: true,
	})
	if err != nil {
		log.Printf("[ERROR] Failed to delete webhook: %v", err)
		return
	}
	log.Printf("[INFO] Previous webhook deleted")
	time.Sleep(3 * time.Second)
	me, err := botAPI.GetMe()
	if err != nil {
		log.Printf("[ERROR] Failed to get bot info: %v", err)
		return
	}
	log.Printf("[INFO] Bot authorized successfully: @%s", me.UserName)
	db, err := sqlx.Connect("postgres", config.Get().DatabaseDSN)
	if err != nil {
		log.Printf("[ERROR] Failed to connect to database: %v", err)
		return
	}
	defer db.Close()

	var (
		articleStorage = storage.NewArticleStorage(db)
		sourceStorage  = storage.NewSourceStorage(db)
		notifier       = notifier.New(
			articleStorage,
			summary.NewOpenAISummarizer(config.Get().OpenAIKey, config.Get().OpenAIModel, config.Get().OpenAIPrompt),
			botAPI,
			config.Get().NotificationInterval,
			2*config.Get().FetchInterval,
			config.Get().TelegramChannelID,
		)
		fetcher = fetcher.New(
			articleStorage,
			sourceStorage,
			config.Get().FetchInterval,
			config.Get().FilterKeywords,
		)
	)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	newsBot := botkit.New(botAPI)
	newsBot.RegisterCmdView("start", bot.ViewCmdStart)
	newsBot.RegisterCmdView("listsources", bot.ViewCmdListSource(sourceStorage))
	newsBot.RegisterCmdView("addsource", bot.ViewCmdAddSource(sourceStorage))
	newsBot.RegisterCmdView("getsource", bot.ViewCmdGetSource(sourceStorage))
	newsBot.RegisterCmdView("deletesource", bot.ViewCmdDeleteSource(sourceStorage))
	newsBot.RegisterCmdView("setpriority", bot.ViewCmdSetPriority(sourceStorage))

	newsBot.RegisterCmdView("findarticles", bot.ViewCmdFindArticles(articleStorage))
	newsBot.RegisterCmdView("publishtochannel", bot.ViewCmdPublishToChannel(
		articleStorage,
		config.Get().TelegramChannelID,
		summary.NewOpenAISummarizer(config.Get().OpenAIKey, config.Get().OpenAIModel, config.Get().OpenAIPrompt),
	))

	openAISummarizer := summary.NewOpenAISummarizer(config.Get().OpenAIKey, config.Get().OpenAIModel, config.Get().OpenAIPrompt)
	newsBot.RegisterCmdView("checkopenai", bot.ViewCmdCheckOpenAI(openAISummarizer))

	newsBot.RegisterCmdView("setopenaikey", bot.ViewCmdSetOpenAIKey())

	commands := []tgbotapi.BotCommand{
		{Command: "start", Description: "Показати довідку та список команд"},
		{Command: "listsources", Description: "Переглянути всі джерела"},
		{Command: "getsource", Description: "Отримати інформацію про джерело за ID"},
		{Command: "addsource", Description: "Додати нове джерело"},
		{Command: "deletesource", Description: "Видалити джерело за ID"},
		{Command: "setpriority", Description: "Встановити пріоритет джерела"},
		{Command: "findarticles", Description: "Знайти статті за вказаний період"},
		{Command: "publishtochannel", Description: "Опублікувати статті в канал"},
		{Command: "checkopenai", Description: "Перевірити статус API ключа OpenAI"},
		{Command: "setopenaikey", Description: "Встановити API ключ OpenAI"},
	}

	if _, err := botAPI.Request(tgbotapi.NewSetMyCommands(commands...)); err != nil {
		log.Printf("[WARN] Failed to set commands: %v", err)
	} else {
		log.Printf("[INFO] Successfully set %d commands", len(commands))
	}

	go func(ctx context.Context) {
		if err := fetcher.Start(ctx); err != nil {
			if !errors.Is(err, context.Canceled) {
				log.Printf("[ERROR] failed to run fetcher: %v", err)
				return
			}
			log.Printf("[INFO] fetcher stopped")
		}
	}(ctx)

	go func(ctx context.Context) {
		if err := notifier.Start(ctx); err != nil {
			if !errors.Is(err, context.Canceled) {
				log.Printf("[ERROR] failed to run notifier: %v", err)
				return
			}
			log.Printf("[INFO] notifier stopped")
		}
	}(ctx)

	log.Printf("[INFO] Starting bot...")

	u := tgbotapi.UpdateConfig{
		Offset:  0,
		Limit:   1,
		Timeout: 5,
	}

	updates, err := botAPI.GetUpdates(u)
	if err != nil {
		log.Printf("[ERROR] Failed to get initial updates: %v", err)
	} else if len(updates) > 0 {
		u.Offset = updates[0].UpdateID + 1
		log.Printf("[INFO] Setting initial offset to %d", u.Offset)
	}

	u.Limit = 100
	u.Timeout = 60

	for {
		select {
		case <-ctx.Done():
			log.Printf("[INFO] Bot stopped due to context cancellation")
			return
		default:
			updates, err := botAPI.GetUpdates(u)
			if err != nil {
				log.Printf("[ERROR] Failed to get updates: %v", err)
				time.Sleep(5 * time.Second)
				continue
			}

			for _, update := range updates {
				if update.UpdateID >= u.Offset {
					u.Offset = update.UpdateID + 1
					log.Printf("[DEBUG] Processing update ID: %d", update.UpdateID)

					if update.Message != nil && update.Message.IsCommand() {
						cmd := update.Message.Command()
						if cmdView, ok := newsBot.GetCmdView(cmd); ok {
							updateCtx, updateCancel := context.WithTimeout(ctx, 5*time.Minute)
							if err := cmdView(updateCtx, botAPI, update); err != nil {
								log.Printf("[ERROR] Failed to execute view for command %s: %v", cmd, err)
							}
							updateCancel()
						}
					}
				}
			}

			time.Sleep(500 * time.Millisecond)
		}
	}
}
