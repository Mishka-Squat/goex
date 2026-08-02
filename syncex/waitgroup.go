package syncex

import (
	"context"
	"sync"
	"time"

	"github.com/Mishka-Squat/goex/contextex"
)

// WaitGroupGo runs multiple functions concurrently and calls waitFn when all are done.
func WaitGroupGo(waitFn func(), fn ...func()) {
	if len(fn) == 0 {
		waitFn()
		return
	}

	var wg sync.WaitGroup

	for _, f := range fn {
		wg.Go(f)
	}

	go func() {
		wg.Wait()
		waitFn()
	}()
}

type CancellableWaitGroup struct {
	sync.WaitGroup
	contextex.CancelContext
}

func (g *CancellableWaitGroup) Ctx() context.Context {
	return g.CancelContext
}

func (g *CancellableWaitGroup) Go(fn func(context.Context) bool) {
	g.WaitGroup.Go(g.Cancellabe(fn))
}

func (g *CancellableWaitGroup) GoWithTimeout(timeout time.Duration, fn func(context.Context) bool) {
	g.WaitGroup.Go(g.CancellabeWithTimeout(timeout, fn))
}

func (g *CancellableWaitGroup) CancelWait() {
	g.CancelContext = g.Cancel()
	g.Wait()
}
