package signals

import (
	"os"
	"os/signal"
)

func WaitForInterrupt() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
}
