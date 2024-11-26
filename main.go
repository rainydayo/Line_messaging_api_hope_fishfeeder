package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"os"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/db"

	"github.com/joho/godotenv"
	"github.com/line/line-bot-sdk-go/v7/linebot"
	"google.golang.org/api/option"
)

var firebaseClient *db.Client

func main() {
	// Load environment variables
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	// Fetch credentials
	lineChannelSecret := os.Getenv("LINE_CHANNEL_SECRET")
	lineAccessToken := os.Getenv("LINE_CHANNEL_ACCESS_TOKEN")
	firebaseCredentials := os.Getenv("FIREBASE_CREDENTIALS")
	firebaseDatabaseURL := os.Getenv("FIREBASE_DATABASE_URL")

	if lineChannelSecret == "" || lineAccessToken == "" || firebaseCredentials == "" || firebaseDatabaseURL == "" {
		log.Fatal("Missing required environment variables in .env file")
	}

	// Decode Firebase credentials
	decodedCredentials, err := base64.StdEncoding.DecodeString(firebaseCredentials)
	if err != nil {
		log.Fatalf("Failed to decode Firebase credentials: %v", err)
	}

	// Create temporary credentials file
	tempFile, err := os.CreateTemp("", "firebase-credentials-*.json")
	if err != nil {
		log.Fatalf("Failed to create temporary file for Firebase credentials: %v", err)
	}
	defer os.Remove(tempFile.Name())

	if _, err := tempFile.Write(decodedCredentials); err != nil {
		log.Fatalf("Failed to write Firebase credentials to temporary file: %v", err)
	}

	// Initialize Firebase app
	ctx := context.Background()
	opt := option.WithCredentialsFile(tempFile.Name())
	app, err := firebase.NewApp(ctx, &firebase.Config{
		DatabaseURL: firebaseDatabaseURL,
	}, opt)
	if err != nil {
		log.Fatalf("Error initializing Firebase app: %v", err)
	}

	// Get Firebase database client
	firebaseClient, err = app.DatabaseWithURL(ctx, firebaseDatabaseURL)
	if err != nil {
		log.Fatalf("Error initializing Firebase database client: %v", err)
	}

	// Initialize LINE Bot client
	bot, err := linebot.New(lineChannelSecret, lineAccessToken)
	if err != nil {
		log.Fatalf("Error initializing LINE bot: %v", err)
	}

	// Start monitoring Firebase database
	monitor := NewMonitor(bot, firebaseClient)
	go monitor.StartMonitoring() // Run in a separate goroutine

	// HTTP handler for LINE Bot
	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		events, err := bot.ParseRequest(r)
		if err != nil {
			if err == linebot.ErrInvalidSignature {
				w.WriteHeader(400)
			} else {
				w.WriteHeader(500)
			}
			return
		}

		for _, event := range events {
			if event.Type == linebot.EventTypeMessage {
				switch message := event.Message.(type) {
				case *linebot.TextMessage:
					handleMessage(bot, event.ReplyToken, message.Text)
				}
			}
		}
	})

	log.Println("Starting server at :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handleMessage(bot *linebot.Client, replyToken, message string) {
	ctx := context.Background()

	switch message {
	case "feed":
		ref := firebaseClient.NewRef("food/state")
		var foodPercentage int
		if err := ref.Get(ctx, &foodPercentage); err != nil {
			bot.ReplyMessage(replyToken, linebot.NewTextMessage("Failed to retrieve food status.")).Do()
			return
		}

		ref = firebaseClient.NewRef("motor/state")
		if foodPercentage > 30 {
			bot.ReplyMessage(replyToken, linebot.NewTextMessage("Too much food!!!")).Do()
		} else {
			if err := ref.Set(ctx, 1); err != nil {
				bot.ReplyMessage(replyToken, linebot.NewTextMessage("Failed to activate the motor for feeding.")).Do()
				return
			}
			bot.ReplyMessage(replyToken, linebot.NewTextMessage("Feeding initiated!")).Do()
		}

	case "Check food status":
		ref := firebaseClient.NewRef("food/state")
		var foodPercentage int
		if err := ref.Get(ctx, &foodPercentage); err != nil {
			bot.ReplyMessage(replyToken, linebot.NewTextMessage("Failed to retrieve food status.")).Do()
			return
		}

		// Respond with the percentage
		message := linebot.NewTextMessage(fmt.Sprintf("Current food level: %d%%", foodPercentage))
		bot.ReplyMessage(replyToken, message).Do()

	default:
		bot.ReplyMessage(replyToken, linebot.NewTextMessage("Send 'feed' to activate feeding or 'Check food status' to check the food level.")).Do()
	}
}

//ngrok http 8080
