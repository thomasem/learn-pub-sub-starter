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

func publishMove(ch *amqp.Channel, move gamelogic.ArmyMove) {
	err := pubsub.PublishJSON(
		ch,
		routing.ExchangePerilTopic,
		fmt.Sprintf("%s.%s", routing.ArmyMovesPrefix, move.Player.Username),
		move,
	)
	if err != nil {
		fmt.Println("Error sending pubsub message:", err)
	}
}

func handlerPaused(gs *gamelogic.GameState) func(routing.PlayingState) pubsub.AckType {
	return func(ps routing.PlayingState) pubsub.AckType {
		defer fmt.Print("> ")
		gs.HandlePause(ps)
		return pubsub.Ack
	}
}

func handlerMove(gs *gamelogic.GameState) func(gamelogic.ArmyMove) pubsub.AckType {
	return func(armyMove gamelogic.ArmyMove) pubsub.AckType {
		defer fmt.Print("> ")
		mo := gs.HandleMove(armyMove)
		switch mo {
		case gamelogic.MoveOutcomeMakeWar, gamelogic.MoveOutComeSafe:
			return pubsub.Ack
		case gamelogic.MoveOutcomeSamePlayer:
			return pubsub.NackDiscard
		default:
			return pubsub.NackDiscard
		}
	}
}

func repl(gs *gamelogic.GameState, ch *amqp.Channel) {
	for {
		words := gamelogic.GetInput()
		if len(words) == 0 {
			continue
		}

		var err error
		switch words[0] {
		case "spawn":
			err = gs.CommandSpawn(words)
		case "move":
			am, err := gs.CommandMove(words)
			if err == nil {
				publishMove(ch, am)
			}
		case "status":
			gs.CommandStatus()
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

	ch, _, err := pubsub.DeclareAndBind(
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

	gs := gamelogic.NewGameState(username)
	err = pubsub.SubscribeJSON(
		conn,
		routing.ExchangePerilDirect,
		fmt.Sprintf("%s.%s", routing.PauseKey, username),
		routing.PauseKey,
		pubsub.Transient,
		handlerPaused(gs),
	)
	if err != nil {
		fmt.Println("Error subscribing to pause queue:", err)
		os.Exit(1)
	}

	err = pubsub.SubscribeJSON(
		conn,
		routing.ExchangePerilTopic,
		fmt.Sprintf("%s.%s", routing.ArmyMovesPrefix, username),
		fmt.Sprintf("%s.*", routing.ArmyMovesPrefix),
		pubsub.Transient,
		handlerMove(gs),
	)
	if err != nil {
		fmt.Println("Error subscribing to army move queue:", err)
		os.Exit(1)
	}

	repl(gs, ch)
}
