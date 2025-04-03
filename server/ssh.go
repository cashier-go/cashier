package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net"

	// "os"
	"time"

	"github.com/cashier-go/cashier/lib"
	"golang.org/x/crypto/ed25519"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

type SSHServer struct {
	config              *ssh.ServerConfig
	tcpListener         net.Listener
	application  *application
}
type SSHServerState struct {
	serverConnection *ssh.ServerConn
	agentForwarded   bool
	agentChannel     *ssh.Channel
	sessionChannel   *ssh.Channel
}

// func New(application application, hostKey []byte, authProvider auth.Provider, keySigner *signer.KeySigner) (*Server, error) {
func newSSHServer(app *application, hostKey []byte) (*SSHServer, error) {
	private, err := ssh.ParsePrivateKey(hostKey)
	if err != nil {
		return nil, fmt.Errorf("failed to parse host key: %w", err)
	}

	config := &ssh.ServerConfig{
		NoClientAuth: false,
		// Only allow keyboard-interactive auth
		KeyboardInteractiveCallback: func(conn ssh.ConnMetadata, client ssh.KeyboardInteractiveChallenge) (*ssh.Permissions, error) {
			return &ssh.Permissions{}, nil
		},
	}
	config.AddHostKey(private)

	return &SSHServer{
		config:              config,
		application: app,
	}, nil
}

func (s *SSHServer) ListenAndServe(listenAddress string) error {
	tcpListener, err := net.Listen("tcp", listenAddress)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", listenAddress, err)
	}
	s.tcpListener = tcpListener

	for {
		tcpConn, err := tcpListener.Accept()
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			continue
		}
		go s.handleTCPConn(tcpConn)
	}
}

func (s *SSHServer) Close() error {
	if s.tcpListener != nil {
		return s.tcpListener.Close()
	}
	return nil
}

func (s *SSHServer) handleTCPConn(tcpConnection net.Conn) {
	defer tcpConnection.Close()

	serverConnection, serverChannels, serverRequests, err := ssh.NewServerConn(tcpConnection, s.config)
	if err != nil {
		log.Printf("Failed to handshake: %v", err)
		return
	}
	defer serverConnection.Close()
	go ssh.DiscardRequests(serverRequests)

	// agentChannel, agentChannelReqs, err := sshConn.OpenChannel("auth-agent@openssh.com", nil)
	// if err != nil {
	// 	log.Printf("Failed to open agent channel: %v", err)
	// 	return
	// }
	// defer agentChannel.Close()
	// go ssh.DiscardRequests(agentChannelReqs)

	serverState := &SSHServerState{
		serverConnection: serverConnection,
		agentForwarded:   false,
		agentChannel:     nil,
		sessionChannel:   nil,
	}

	for newChannel := range serverChannels {
		if newChannel.ChannelType() != "session" {
			newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}

		sessionChannel, requests, err := newChannel.Accept()
		if err != nil {
			log.Printf("Failed to accept channel: %v", err)
			continue
		}

		serverState.sessionChannel = &sessionChannel
		go s.handleSessionChannel(sessionChannel, requests, serverState)
	}
}

func (s *SSHServer) handleSessionChannel(channel ssh.Channel, requests <-chan *ssh.Request, serverState *SSHServerState) {
	defer channel.Close()

	for req := range requests {
		switch req.Type {
		case "shell":
			req.Reply(true, nil)
			go s.handleShellChannel(channel, serverState)
		case "auth-agent-req@openssh.com":
			serverState.agentForwarded = true
			req.Reply(true, nil)
		default:
			req.Reply(false, nil)
		}
	}
}

func (s *SSHServer) injectKeyToAgent(serverState *SSHServerState, privateKey interface{}, certificate *ssh.Certificate) error {
	if !serverState.agentForwarded {
		return fmt.Errorf("no SSH agent forwarded")
	}

	// Open new channel for agent forwarding
	agentChannel, _, err := serverState.serverConnection.OpenChannel("auth-agent@openssh.com", nil)
	if err != nil {
		return fmt.Errorf("failed to open agent channel: %w", err)
	}
	defer agentChannel.Close()

	// Create agent client from channel
	agentClient := agent.NewClient(&channelAgent{agentChannel})

	// Add the key to the agent
	err = agentClient.Add(agent.AddedKey{
		PrivateKey:   privateKey,
		Certificate:  certificate,
		Comment:      "cashier-generated key",
		LifetimeSecs: uint32(24 * 60 * 60), // 24 hours
	})
	if err != nil {
		return fmt.Errorf("failed to add key to agent: %w", err)
	}

	return nil
}

type channelAgent struct {
	channel ssh.Channel
}

func (c *channelAgent) Read(data []byte) (n int, err error) {
	return c.channel.Read(data)
}

func (c *channelAgent) Write(data []byte) (n int, err error) {
	return c.channel.Write(data)
}

func (s *SSHServer) handleShellChannel(channel ssh.Channel, serverState *SSHServerState) {
	defer channel.Close()
	sshUsername := serverState.serverConnection.User()

	io.WriteString(channel, "## Welcome to Cashier SSH Certificate Service\n")

	if !serverState.agentForwarded {
		io.WriteString(channel, "Agent forwarding is not enabled.\n")
		io.WriteString(channel, "Please enable it to enable injecting the resulting certificate.\n")
		return
	}

	randomBuffer := make([]byte, 32)
	io.ReadFull(rand.Reader, randomBuffer)
	randomStateToken := hex.EncodeToString(randomBuffer)
	loginURL := s.application.config.PublicURLBase + "/auth/login?state=" + randomStateToken

	waitTime := 10 * time.Minute
	callbackChannel := s.application.authCallbackManager.RegisterSessionWithTTL(randomStateToken, waitTime)

	io.WriteString(channel, "# Log In Here: ")
	io.WriteString(channel, loginURL)
	io.WriteString(channel, "\n")
	io.WriteString(channel, "# Waiting on completion ... \n")

	select {
	case authCallback := <-callbackChannel:
		ctx := context.Background()
		authProviderUsername := s.application.authprovider.Username(ctx, authCallback.Token)
		if authProviderUsername == sshUsername {
			io.WriteString(channel, "# Authentication successful!\n")
		} else {
			io.WriteString(channel, "# Error: Your requested SSH username (\"")
			io.WriteString(channel, sshUsername)
			io.WriteString(channel, "\") doesn't match your username from your auth provider (\"")
			io.WriteString(channel, authProviderUsername)
			io.WriteString(channel, "\")\n")
			return
		}
	case <-time.After(waitTime):
		io.WriteString(channel, "# Error: Authentication timed out.\n")
		return
	}

	// Generate new ed25519 key
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		log.Printf("Failed to generate key: %v", err)
		return
	}

	// Convert to SSH format
	sshPublicKey, err := ssh.NewPublicKey(publicKey)
	if err != nil {
		log.Printf("Failed to convert public key: %v", err)
		return
	}

	signRequest := lib.SignRequest{
		Key:        string(ssh.MarshalAuthorizedKey(sshPublicKey)),
		ValidUntil: time.Now().Add(9 * time.Hour),
	}

	signedCert, err := s.application.keysigner.SignUserKey(&signRequest, sshUsername)
	if err != nil {
		log.Printf("Failed to sign user cert: %v", err)
	}

	if err := s.injectKeyToAgent(serverState, privateKey, signedCert); err != nil {
		log.Printf("Failed to inject key to agent: %v", err)
		io.WriteString(channel, fmt.Sprintf("Error injecting key to agent: %v\n", err))
		return
	}

	io.WriteString(channel, "Successfully generated and added certificate to your agent\n")
}
