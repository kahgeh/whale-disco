package ctx

import (
	"context"
	"github.com/kahgeh/whale-disco/pkg/logger"
	"os"
	"os/signal"
)

type ConsoleAppContext struct {
	ctx       context.Context
	cancel    context.CancelFunc
	osSigChan chan os.Signal
	cb        func()
}

var consoleAppCtx *ConsoleAppContext

func init() {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	osSigChan := make(chan os.Signal, 1)

	consoleAppCtx = &ConsoleAppContext{ctx: ctx,
		cancel:    cancel,
		osSigChan: osSigChan}

	signal.Notify(osSigChan, os.Interrupt)
	// trap Ctrl+C and call cancel on the context
}

func GetContext() context.Context {
	return consoleAppCtx.ctx
}

func SetCallback(cb func()) {
	consoleAppCtx.cb = cb
}

func WaitOnCtrlCSignalOrCompletion() {
	log := logger.New("WaitOnCtrlCSignalOrCompletion")
	defer log.LogDone()
	select {
	case <-consoleAppCtx.osSigChan:
		log.Info("user triggered termination")
		consoleAppCtx.cancel()
		consoleAppCtx.cb()
	case <-consoleAppCtx.ctx.Done():
	}
}

func CleanUp() {
	consoleAppCtx.cleanUp()
}

func (consoleAppCtx *ConsoleAppContext) cleanUp() {
	signal.Stop(consoleAppCtx.osSigChan)
	consoleAppCtx.cancel()
}
