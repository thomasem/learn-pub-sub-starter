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

func safePublishJSON[T any](ch *amqp.Channel, exchange, key string, val T) {
	err := pubsub.PublishJSON(
		ch,
		exchange,
		key,
		val,
	)
	if err != nil {
		fmt.Println("Error sending pubsub message:", err)
	}
}

func safePublishPaused(ch *amqp.Channel, paused bool) {
	safePublishJSON(
		ch,
		routing.ExchangePerilDirect,
		routing.PauseKey,
		routing.PlayingState{IsPaused: paused},
	)
}

func repl(ch *amqp.Channel) {
	gamelogic.PrintServerHelp()
	for {
		input := gamelogic.GetInput()
		if len(input) == 0 {
			continue
		}

		switch input[0] {
		case "quit":
			gamelogic.PrintQuit()
			os.Exit(0)
		case "help":
			gamelogic.PrintServerHelp()
		case "pause":
			safePublishPaused(ch, true)
		case "resume":
			safePublishPaused(ch, false)
		}
	}
}

func main() {
	fmt.Println("Startng Peril server...")

	url := "amqp://guest:guest@localhost:5672/"
	conn, err := amqp.Dial(url)
	if err != nil {
		fmt.Println("Error dialing:", err)
		os.Exit(1)
	}

	defer func() {
		conn.Close()
		fmt.Println("Closed RabbitMQ connection.")
	}()

	fmt.Println("Connected to RabbitMQ. Press Ctrl + C or enter 'quit' to exit.")

	ch, err := conn.Channel()
	if err != nil {
		fmt.Println("Error opening channel:", err)
		os.Exit(1)
	}

	_, _, err = pubsub.DeclareAndBind(
		conn,
		routing.ExchangePerilTopic,
		routing.GameLogSlug,
		fmt.Sprintf("%s.*", routing.GameLogSlug),
		pubsub.Durable,
	)
	if err != nil {
		fmt.Println("Error declaring and binding log queue:", err)
		os.Exit(1)
	}
	go func() {
		signals.WaitForInterrupt()
		// Maybe send pause or some message to client?
		fmt.Println("\nInterrupt detected. Exiting...")
		os.Exit(0)
	}()

	repl(ch)
}
