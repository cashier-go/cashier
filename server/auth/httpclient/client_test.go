package httpclient

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/assert"
	"golang.org/x/oauth2"
)

func Test_janitor(t *testing.T) {
	tests := []struct {
		name  string
		token *oauth2.Token
		want  bool
	}{
		{
			name: "entry is present",
			token: &oauth2.Token{
				Expiry: time.Now().Add(1 * time.Hour),
			},
			want: true,
		},
		{
			name: "entry is removed",
			token: &oauth2.Token{
				Expiry: time.Now().Add(-1 * time.Hour),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testcache := New(0)
			testcache.clients[tt.token] = &http.Client{}
			testcache.cleanup()
			_, got := testcache.clients[tt.token]
			if tt.want != got {
				t.Errorf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestCacheCaches(t *testing.T) {
	cache := New(time.Second)
	close(cache.stopJanitor)
	token1 := &oauth2.Token{
		AccessToken: "abc",
	}
	token2 := &oauth2.Token{
		AccessToken: "def",
	}
	// Create two clients, verify they're cached.
	cache.Get(context.TODO(), &oauth2.Config{}, token1)
	cache.Get(context.TODO(), &oauth2.Config{}, token2)
	assert.Len(t, cache.clients, 2)

	client1, ok := cache.clients[token1]
	assert.True(t, ok)
	assert.NotNil(t, client1)
	client2, ok := cache.clients[token2]
	assert.True(t, ok)
	assert.NotNil(t, client2)
	assert.NotEqual(t, client1, client2)
}

func TestCacheDestroy(t *testing.T) {
	cache := New(time.Second)
	close(cache.stopJanitor)
	token1 := &oauth2.Token{
		AccessToken: "abc",
	}
	token2 := &oauth2.Token{
		AccessToken: "def",
	}
	// Create two clients, verify they're cached.
	cache.Get(context.TODO(), &oauth2.Config{}, token1)
	cache.Get(context.TODO(), &oauth2.Config{}, token2)
	assert.Len(t, cache.clients, 2)

	// Destroy one entry, verify that the other is untouched
	cache.Destroy(token1)
	client1, ok := cache.clients[token1]
	assert.False(t, ok)
	assert.Nil(t, client1)
	client2, ok := cache.clients[token2]
	assert.True(t, ok)
	assert.NotNil(t, client2)
}

func TestCacheItems(t *testing.T) {
	cache := New(time.Second)
	close(cache.stopJanitor)
	token := &oauth2.Token{
		AccessToken: "abc",
	}
	client1 := cache.Get(context.TODO(), &oauth2.Config{}, token)
	assert.Len(t, cache.clients, 1)

	cache.Destroy(token)
	otherClient1 := cache.Get(context.TODO(), &oauth2.Config{}, token)
	spew.Dump(client1)
	spew.Dump(otherClient1)
	assert.NotSame(t, client1, otherClient1)
}
