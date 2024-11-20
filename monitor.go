package main

import (
	"context"
	"log"
	"time"

	"firebase.google.com/go/v4/db"
	"github.com/line/line-bot-sdk-go/v7/linebot"
)

type Monitor struct {
	Bot            *linebot.Client
	FirebaseClient *db.Client
	PreviousState  State
}

type State struct {
	Temp struct {
		State int `json:"state"`
	} `json:"temp"`
	Quality struct {
		State int `json:"state"`
	} `json:"quality"`
}

func NewMonitor(bot *linebot.Client, firebaseClient *db.Client) *Monitor {
	return &Monitor{
		Bot:            bot,
		FirebaseClient: firebaseClient,
	}
}

func (m *Monitor) StartMonitoring() {
	ctx := context.Background()
	ref := m.FirebaseClient.NewRef("/")

	for {
		var currentState State
		if err := ref.Get(ctx, &currentState); err != nil {
			log.Printf("Error reading database: %v", err)
			time.Sleep(30 * time.Second)
			continue
		}
		m.checkAndNotify(currentState)
		m.PreviousState = currentState
		time.Sleep(30 * time.Second)
	}
}

func (m *Monitor) checkAndNotify(current State) {
	ctx := context.Background()

	// Check temperature state changes
	if current.Temp.State != m.PreviousState.Temp.State {
		switch current.Temp.State {
		case 0:
			m.sendNotification(ctx, "Temperature is too low!")
		case 1:
			m.sendNotification(ctx, "Temperature is now okay.")
		case 2:
			m.sendNotification(ctx, "Temperature is too high!")
		}
	}

	// Check quality state changes
	if current.Quality.State != m.PreviousState.Quality.State {
		switch current.Quality.State {
		case 0, 2:
			m.sendNotification(ctx, "Quality is too low!")
		case 1:
			m.sendNotification(ctx, "Quality is now okay.")
		}
	}
}

func (m *Monitor) sendNotification(ctx context.Context, message string) {
	if _, err := m.Bot.BroadcastMessage(linebot.NewTextMessage(message)).Do(); err != nil {
		log.Printf("Failed to send notification: %v\n", err)
	} else {
		log.Printf("Notification sent: %s\n", message)
	}
}
