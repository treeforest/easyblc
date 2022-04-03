package graceful

import (
	"os"
	"os/signal"
	"time"
)

func Stop(fn func()) {
	// graceful stop
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, os.Kill)
	<-done
	fn()
}

func StopWithTime(duration time.Duration, fn func()) {
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, os.Kill)
	<-done
	fn()
	time.Sleep(duration)
}
