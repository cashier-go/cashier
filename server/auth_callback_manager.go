package server

import (
	"time"
	"sync"
	"golang.org/x/oauth2"
)

type AuthCallback struct {
	Token *oauth2.Token
}
type SessionInfo struct {
	Channel   chan AuthCallback
	ExpiresAt time.Time
}

type AuthCallbackManager struct {
	sessions     map[string]SessionInfo
	sessionsLock sync.RWMutex
	defaultTTL   time.Duration
	done         chan struct{}
}

func NewAuthcallbackManager(defaultTTL time.Duration) *AuthCallbackManager {
	acm := &AuthCallbackManager{
		sessions: make(map[string]SessionInfo),
		defaultTTL: defaultTTL,
		done: make(chan struct{}),
	}
	go acm.cleanupExpiredSessions()
	return acm
}
func (acm *AuthCallbackManager) cleanupExpiredSessions() {
	ticker := time.NewTicker(1*time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			acm.cleanup()
		case <-acm.done:
			return
		}
	}
}

func (acm *AuthCallbackManager) cleanup() {
	now := time.Now()
	var expiredKeys []string

	acm.sessionsLock.RLock()
	for key, info := range acm.sessions {
		if now.After(info.ExpiresAt) {
			expiredKeys = append(expiredKeys, key)
		}
	}
	acm.sessionsLock.RUnlock()

	if len(expiredKeys) > 0 {
		acm.sessionsLock.Lock()
		for _, key := range expiredKeys {
			info, exists := acm.sessions[key]
			if exists && now.After(info.ExpiresAt) {
				close(info.Channel)
				delete(acm.sessions, key)
			}
		}
		acm.sessionsLock.Unlock()
	}
}

func (acm *AuthCallbackManager) RegisterSession(state string) chan AuthCallback {
	return acm.RegisterSessionWithTTL(state, acm.defaultTTL)
}

func (acm *AuthCallbackManager) RegisterSessionWithTTL(state string, ttl time.Duration) chan AuthCallback {
	callbackChan := make(chan AuthCallback)
	expiresAt := time.Now().Add(ttl)
	acm.sessionsLock.Lock()
	acm.sessions[state] = SessionInfo{
		Channel: callbackChan,
		ExpiresAt: expiresAt,
	}
	acm.sessionsLock.Unlock()
	return callbackChan
}

func (acm *AuthCallbackManager) UnregisterSession(state string) {
	acm.sessionsLock.Lock()
	if info, exists := acm.sessions[state]; exists {
		close(info.Channel)
		delete(acm.sessions, state)
	}
	acm.sessionsLock.Unlock()
}

func (acm *AuthCallbackManager) HandleCallback(state string, token *oauth2.Token) bool {
	acm.sessionsLock.RLock()
	info, exists := acm.sessions[state]
	acm.sessionsLock.RUnlock()

	if !exists || time.Now().After(info.ExpiresAt) {
		if exists {
			acm.UnregisterSession(state)
		}
		return false
	}

	select {
	case info.Channel <- AuthCallback{Token: token}:
		return true
	default:
		acm.UnregisterSession(state)
		return false
	}
}

func (acm *AuthCallbackManager) Shutdown() {
	close(acm.done)
	acm.sessionsLock.Lock()
	for _, info := range acm.sessions {
		close(info.Channel)
	}
	acm.sessions = make(map[string]SessionInfo)
	acm.sessionsLock.Unlock()
}
