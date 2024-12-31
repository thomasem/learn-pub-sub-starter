package main

import (
	"fmt"
	"os"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/signals"
)

func publishMove(ch *amqp.Channel, key string, move gamelogic.ArmyMove) {
	err := pubsub.PublishJSON(
		ch,
		routing.ExchangePerilTopic,
		key,
		move,
	)
	if err != nil {
		fmt.Println("Error sending move to pubsub:", err)
	}
}

func publishLog(ch *amqp.Channel, log routing.GameLog) error {
	return pubsub.PublishGob(
		ch,
		routing.ExchangePerilTopic,
		fmt.Sprintf("%s.%s", routing.GameLogSlug, log.Username),
		log,
	)
}

func handlerPaused(gs *gamelogic.GameState) func(routing.PlayingState) pubsub.AckType {
	return func(ps routing.PlayingState) pubsub.AckType {
		defer fmt.Print("> ")
		gs.HandlePause(ps)
		return pubsub.Ack
	}
}

func handlerMove(gs *gamelogic.GameState, ch *amqp.Channel) func(gamelogic.ArmyMove) pubsub.AckType {
	return func(am gamelogic.ArmyMove) pubsub.AckType {
		defer fmt.Print("> ")
		mo := gs.HandleMove(am)
		switch mo {
		case gamelogic.MoveOutcomeMakeWar:
			err := pubsub.PublishJSON(
				ch,
				routing.ExchangePerilTopic,
				fmt.Sprintf("%s.%s", routing.WarRecognitionsPrefix, am.Player.Username),
				gamelogic.RecognitionOfWar{
					Attacker: am.Player,
					Defender: gs.Player,
				},
			)
			if err != nil {
				fmt.Println("Error sending war recognition:", err)
				return pubsub.NackRequeue
			}
			return pubsub.Ack
		case gamelogic.MoveOutComeSafe:
			return pubsub.Ack
		case gamelogic.MoveOutcomeSamePlayer:
			return pubsub.NackDiscard
		default:
			return pubsub.NackDiscard
		}
	}
}

func handlerWarRecognition(gs *gamelogic.GameState, ch *amqp.Channel) func(gamelogic.RecognitionOfWar) pubsub.AckType {
	return func(warRec gamelogic.RecognitionOfWar) pubsub.AckType {
		defer fmt.Print("> ")
		outcome, winner, loser := gs.HandleWar(warRec)
		switch outcome {
		case gamelogic.WarOutcomeNotInvolved:
			return pubsub.NackRequeue
		case gamelogic.WarOutcomeNoUnits:
			return pubsub.NackDiscard
		case gamelogic.WarOutcomeOpponentWon, gamelogic.WarOutcomeYouWon:
			err := publishLog(ch, routing.GameLog{
				CurrentTime: time.Now(),
				Message:     fmt.Sprintf("%s won a war against %s", winner, loser),
				Username:    gs.Player.Username,
			})
			if err != nil {
				fmt.Println("Error sending game log:", err)
				return pubsub.NackRequeue
			}
			return pubsub.Ack
		case gamelogic.WarOutcomeDraw:
			err := publishLog(ch, routing.GameLog{
				CurrentTime: time.Now(),
				Message:     fmt.Sprintf("A war between %s and %s resulted in a draw", winner, loser),
				Username:    gs.Player.Username,
			})
			if err != nil {
				fmt.Println("Error sending game log:", err)
				return pubsub.NackRequeue
			}
			return pubsub.Ack
		default:
			fmt.Println("Unknown war outcome:", outcome)
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
				publishMove(ch, fmt.Sprintf("%s.%s", routing.ArmyMovesPrefix, am.Player.Username), am)
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
		handlerMove(gs, ch),
	)
	if err != nil {
		fmt.Println("Error subscribing to army move queue:", err)
		os.Exit(1)
	}

	err = pubsub.SubscribeJSON(
		conn,
		routing.ExchangePerilTopic,
		routing.WarRecognitionsPrefix,
		fmt.Sprintf("%s.*", routing.WarRecognitionsPrefix),
		pubsub.Durable,
		handlerWarRecognition(gs, ch),
	)
	if err != nil {
		fmt.Println("Error subscribing to war queue:", err)
		os.Exit(1)
	}

	repl(gs, ch)
}
