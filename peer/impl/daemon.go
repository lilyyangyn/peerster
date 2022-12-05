package impl

import (
	"context"
	"time"

	"go.dedis.ch/cs438/types"
)

// MessagingDaemon starts a new loop to listen to the message
func (n *node) MessagingDaemon(ctx context.Context) error {
	go func() {
	out:
		for {
			select {
			case <-ctx.Done():
				// use context to determine when to stop the goroutine
				break out
			default:
				pkt, err := n.conf.Socket.Recv(ReadTimeout)
				if err != nil {
					continue
				}
				err = n.ProcessPkt(pkt)
				if err != nil {
					// return
					continue
				}
			}
		}
	}()

	return nil
}

// HeartBeatMecahnism implements heartbeat mechanism to periodically notify self
func (n *node) HeartBeatDaemon(ctx context.Context, interval time.Duration) error {
	if interval == 0 {
		// the heartbeat mechanism must not be activated
		return nil
	}
	heartbeatTicker := time.NewTicker(interval)
	err := n.SendHeartbeatMessage(types.EmptyMessage{})
	if err != nil {
		return err
	}

	go func() {
	out:
		for {
			select {
			case <-ctx.Done():
				heartbeatTicker.Stop()
				break out
			case <-heartbeatTicker.C:
				err := n.SendHeartbeatMessage(types.EmptyMessage{})
				if err != nil {
					continue
				}
			}
		}
	}()

	return nil
}

// AntiEntropyMechanism implements anti-entropy mechanism for gossip sync
func (n *node) AntiEntropyDaemon(ctx context.Context, interval time.Duration) error {
	if interval == 0 {
		// the anti-entropy mechanism must not be activated
		return nil
	}
	antiEntropyTicker := time.NewTicker(interval)

	go func() {
	out:
		for {
			select {
			case <-ctx.Done():
				antiEntropyTicker.Stop()
				break out
			case <-antiEntropyTicker.C:
				neighbor, ok := n.GetRandomNeighbor(map[string]struct{}{})
				if !ok {
					// no available neighbor
					continue
				}
				err := n.SendStatusMessage(neighbor)
				if err != nil {
					continue
				}
			}
		}
	}()

	return nil
}
