/*
sometimes we have clients connecting to a single server to "listen" to new events, this broadcaster helps to implement that.
a target may register itself to listen for new data
the producer calls NewData when it has new data
*/
package broadcaster

import (
	"fmt"
	"io"
	"sync"
)

type Broadcaster struct {
	listeners []*broadcastListener
	lock      sync.Mutex
}

func (b *Broadcaster) NewData(data any) {
	x := b.listeners
	for _, bl := range x {
		select {
		case bl.ch <- data:
			//
		default:
			fmt.Printf("[broadcaster] failed to send to listener\n")
		}
	}
}

func (b *Broadcaster) Handle(i any, f func(target any, data any) error) error {
	bl := &broadcastListener{srv: i, f: f, ch: make(chan any, 10)}
	b.lock.Lock()
	b.listeners = append(b.listeners, bl)
	b.lock.Unlock()
	var err error
	for {
		data := <-bl.ch
		err := bl.f(bl.srv, data)
		if err != nil {
			break
		}
	}
	b.lock.Lock()
	var n []*broadcastListener
	for _, blx := range b.listeners {
		if bl == blx {
			continue
		}
		n = append(n, blx)
	}
	b.listeners = n
	b.lock.Unlock()

	if err == io.EOF {
		return nil
	}
	return err

}

type broadcastListener struct {
	srv any
	f   func(target any, data any) error
	ch  chan any
}
