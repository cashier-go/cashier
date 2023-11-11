package httpclient

import (
	"context"
	"net/http"
	"sync"
	"time"

	"golang.org/x/oauth2"
)

type ClientCache struct {
	mu          sync.Mutex
	clients     map[*oauth2.Token]*http.Client
	stopJanitor chan struct{}
}

func New(cleanupDuration time.Duration) *ClientCache {
	cache := &ClientCache{
		clients:     make(map[*oauth2.Token]*http.Client),
		stopJanitor: make(chan struct{}, 1),
	}
	go cache.janitor(cleanupDuration)
	return cache
}

// cleanup removes any expired clients from the cache
func (cc *ClientCache) cleanup() {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	for token := range cc.clients {
		if token.Expiry.Before(time.Now()) {
			delete(cc.clients, token)
		}
	}
}

// janitor runs periodically to cleanup any expired tokens from the cache
func (cc *ClientCache) janitor(cleanupDuration time.Duration) {
	if cleanupDuration <= 0 {
		return
	}
	ticker := time.NewTicker(cleanupDuration)
	defer ticker.Stop()
	for {
		select {
		case <-cc.stopJanitor:
			return
		case <-ticker.C:
			cc.cleanup()
		}
	}
}

// Get returns an oauth2 http client, either from cache or creating a new one
func (cc *ClientCache) Get(ctx context.Context, config *oauth2.Config, token *oauth2.Token) *http.Client {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	if client, ok := cc.clients[token]; ok {
		return client
	}
	client := config.Client(ctx, token)
	cc.clients[token] = client
	return client
}

// Destroy removes an oauth2 http client from the cache
func (cc *ClientCache) Destroy(token *oauth2.Token) {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	delete(cc.clients, token)
}
