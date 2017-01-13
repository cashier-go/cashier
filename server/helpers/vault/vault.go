package vault

import (
	"fmt"
	"strings"

	"github.com/hashicorp/vault/api"
)

// NewClient returns a new vault client.
func NewClient(address, token string) (*Client, error) {
	config := &api.Config{
		Address: address,
	}
	client, err := api.NewClient(config)
	if err != nil {
		return nil, err
	}
	client.SetToken(token)
	return &Client{
		vault: client,
	}, nil
}

func parseName(name string) (path, key string) {
	name = strings.TrimPrefix(name, "/vault/")
	i := strings.LastIndex(name, "/")
	if i < 0 {
		return name, ""
	}
	return name[:i], name[i+1:]
}

// Client is a simple client for vault.
type Client struct {
	vault *api.Client
}

// Read returns a secret for a given path and key of the form `/vault/secret/path/key`.
// If the requested key cannot be read the original string is returned along with an error.
func (c *Client) Read(value string) (string, error) {
	p, k := parseName(value)
	data, err := c.vault.Logical().Read(p)
	if err != nil {
		return value, err
	}
	if data == nil {
		return value, fmt.Errorf("no such key %s", k)
	}
	secret, ok := data.Data[k]
	if !ok {
		return value, fmt.Errorf("no such key %s", k)
	}
	return secret.(string), nil
}

// Delete deletes the secret from vault.
func (c *Client) Delete(value string) error {
	p, _ := parseName(value)
	_, err := c.vault.Logical().Delete(p)
	return err
}
