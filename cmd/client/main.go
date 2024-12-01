package main

import (
	"fmt"
	"os"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/signals"
)

func main() {
	fmt.Println("Starting Peril client...")

	url := "amqp://guest:guest@localhost:5672/"
	conn, err := amqp.Dial(url)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	defer func() {
		conn.Close()
		fmt.Println("Closed RabbitMQ connection.")
	}()

	fmt.Println("Connected to RabbitMQ. Press Ctrl + C to exit.")

	username, err := gamelogic.ClientWelcome()
	if err != nil {
		fmt.Println("Error getting username:", err)
		os.Exit(1)
	}

	_, _, err = pubsub.DeclareAndBind(
		conn,
		routing.ExchangePerilDirect,
		fmt.Sprintf("%s.%s", routing.PauseKey, username),
		routing.PauseKey,
		pubsub.Transient,
	)
	if err != nil {
		fmt.Println("Error declaring queue:", err)
		os.Exit(1)
	}

	go func() {
		signals.WaitForInterrupt()
		fmt.Println("\nInterrupt detected. Exiting...")
		os.Exit(0)
	}()

	gameState := gamelogic.NewGameState(username)
	for {
		words := gamelogic.GetInput()
		if len(words) == 0 {
			continue
		}

		var err error
		switch words[0] {
		case "spawn":
			err = gameState.CommandSpawn(words)
		case "move":
			_, err = gameState.CommandMove(words)
		case "status":
			gameState.CommandStatus()
		case "spam":
			fmt.Println("Spamming not allowed yet!")
		case "quit":
			gamelogic.PrintQuit()
			os.Exit(0)
		case "help":
			gamelogic.PrintClientHelp()
		default:
			fmt.Println("Unknown command:", words[0])
			gamelogic.PrintClientHelp()
		}
		if err != nil {
			fmt.Println("Error:", err)
		}
	}
}
