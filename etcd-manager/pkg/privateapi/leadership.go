/*
Copyright 2020 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package privateapi

import (
	"context"
	crypto_rand "crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"time"

	grpccontext "golang.org/x/net/context"
	"k8s.io/klog/v2"
)

type leadership struct {
	notification *LeaderNotificationRequest
	timestamp    time.Time
}

func (s *Server) LeaderNotification(ctx grpccontext.Context, request *LeaderNotificationRequest) (*LeaderNotificationResponse, error) {
	klog.V(3).Infof("Got LeaderNotification %s", request)

	if request.View == nil {
		return nil, fmt.Errorf("View is required")
	}
	if len(request.View.Healthy) == 0 {
		return nil, fmt.Errorf("View.Healthy is required")
	}

	s.addPeersFromView(request.View)

	s.mutex.Lock()
	defer s.mutex.Unlock()

	viewMap := make(map[PeerId]*PeerInfo)
	for _, p := range request.View.Healthy {
		peerId := PeerId(p.Id)
		viewMap[peerId] = p
	}

	reject := false
	for id, peer := range s.peers {
		leaderView := viewMap[id]
		if leaderView != nil {
			continue
		}
		_, healthy := peer.status(s.HealthyTimeout)
		if healthy {
			klog.Warningf("LeaderElection view did not include peer %s; will reject", id)
			reject = true
			break
		}
	}

	response := &LeaderNotificationResponse{}

	if reject {
		response.Accepted = false
		response.View = &View{}
		for _, peer := range s.peers {
			peerInfo, healthy := peer.status(s.HealthyTimeout)
			if healthy {
				response.View.Healthy = append(response.View.Healthy, peerInfo)
			}
		}
		return response, nil
	}

	response.Accepted = true
	s.leadership = &leadership{
		notification: request,
		timestamp:    time.Now(),
	}
	return response, nil
}

func (s *Server) snapshotHealthy() (map[PeerId]*peer, map[PeerId]*PeerInfo) {
	snapshot := make(map[PeerId]*peer)
	infos := make(map[PeerId]*PeerInfo)

	s.mutex.Lock()
	defer s.mutex.Unlock()
	for id, peer := range s.peers {
		info, healthy := peer.status(s.HealthyTimeout)
		if healthy {
			snapshot[id] = peer
			infos[id] = info
		}
	}
	return snapshot, infos
}

func (s *Server) addPeersFromView(view *View) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for _, p := range view.Healthy {
		peerId := PeerId(p.Id)

		existing := s.peers[peerId]
		if existing == nil {
			klog.Infof("leader notification found new candidate peer: %s", peerId)
			existing = &peer{
				server: s,
				id:     peerId,
			}
			s.peers[peerId] = existing
			existing.updatePeerInfo(p)
			go existing.Run(s.context, s.PingInterval)
		}
	}
}

func (s *Server) IsLeader(leadershipToken string) bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// TODO: Immediately depose leader if we see a lower peer?

	if s.leadership == nil {
		klog.Infof("will reject leadership token %q: no leadership", leadershipToken)
		return false
	}
	if s.leadership.notification == nil {
		klog.Infof("will reject leadership token %q: no leadership notification", leadershipToken)
		return false
	}
	if s.leadership.notification.View == nil {
		klog.Infof("will reject leadership token %q: no leadership notification view", leadershipToken)
		return false
	}
	if s.leadership.notification.View.LeadershipToken != leadershipToken {
		klog.Infof("will reject leadership token %q: actual leadership is %q", leadershipToken, s.leadership.notification.View.LeadershipToken)
		return false
	} else {
		return true
	}
}

func (s *Server) BecomeLeader(ctx context.Context) ([]PeerId, string, error) {
	// TODO: Should we send a notification if we ourselves would reject it?
	snapshot, infos := s.snapshotHealthy()

	request := &LeaderNotificationRequest{}

	request.View = &View{}
	for _, info := range infos {
		request.View.Healthy = append(request.View.Healthy, info)
	}
	request.View.Leader = &s.myInfo
	request.View.LeadershipToken = randomToken()

	var acked []PeerId

	for peerId := range snapshot {
		conn, err := s.GetPeerClient(peerId)
		if err != nil {
			return nil, "", fmt.Errorf("error getting peer client for %s: %v", peerId, err)
		}

		peerClient := NewClusterServiceClient(conn)

		response, err := peerClient.LeaderNotification(ctx, request)
		if err != nil {
			return nil, "", fmt.Errorf("error sending leader notification to %s: %v", peerId, err)
		}

		if response.View != nil {
			s.addPeersFromView(response.View)
		}

		if !response.Accepted {
			return nil, "", fmt.Errorf("our leadership bid was not accepted by peer %q: %v", peerId, response)
		}
		acked = append(acked, peerId)
	}

	return acked, request.View.LeadershipToken, nil
}

func (s *Server) AssertLeadership(ctx context.Context, leadershipToken string) error {
	// TODO: Should we send a notification if we ourselves would reject it?
	snapshot, infos := s.snapshotHealthy()

	request := &LeaderNotificationRequest{}

	request.View = &View{}
	for _, info := range infos {
		request.View.Healthy = append(request.View.Healthy, info)
	}
	request.View.Leader = &s.myInfo
	request.View.LeadershipToken = leadershipToken

	for peerID := range snapshot {
		conn, err := s.GetPeerClient(peerID)
		if err != nil {
			return fmt.Errorf("error getting peer client for %q: %w", peerID, err)
		}

		peerClient := NewClusterServiceClient(conn)

		timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		response, err := peerClient.LeaderNotification(timeoutCtx, request)
		cancel()
		if err != nil {
			return fmt.Errorf("error sending leader assertion to %q: %w", peerID, err)
		}

		if response.View != nil {
			s.addPeersFromView(response.View)
		}

		if !response.Accepted {
			return fmt.Errorf("our leadership assertion was not accepted by peer %q: %v", peerID, response)
		}
	}

	return nil
}

func randomToken() string {
	b := make([]byte, 16)
	_, err := io.ReadFull(crypto_rand.Reader, b)
	if err != nil {
		klog.Fatalf("error generating random token: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
