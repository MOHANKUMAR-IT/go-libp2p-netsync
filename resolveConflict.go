package netsync

import (
	"context"
	"github.com/MOHANKUMAR-IT/go-libp2p-netsync/pb"
	"sync"
	"time"
)

func (ns *NetSyncService) resolveConflict(ctx context.Context, msg *pb.NetSyncMessage) bool {
	sctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	peers, err := ns.getSortedClosestPeers(sctx, msg.Key)
	if err != nil {
		return true // can be no connected peers
	}

	conflictWithPeers := 0

	msg.LockState = pb.LockState_LOCK_TRY_RS_ACQUIRE

	wg := sync.WaitGroup{}
	for _, p := range peers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sctx, cancel := context.WithTimeout(ns.ctx, time.Second*5)
			defer cancel()
			response, err := ns.contactPeer(sctx, p, msg)
			if err != nil {
				return
			}
			if response.LockState != pb.LockState_LOCK_ACQUIRED {
				conflictWithPeers++
				time.Sleep(time.Second * 2)
			}
		}()
	}

	wg.Wait()

	canAllow := conflictWithPeers < int(float64(len(peers))*0.6)

	if conflictWithPeers > 0 && canAllow {
		logger.Debugf("Conflict resolved with %d peers , but allowed", conflictWithPeers)
	}

	return canAllow

}
