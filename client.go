package sshrpc

import (
	"net/rpc"

	"golang.org/x/crypto/ssh"
)

// Client represents an RPC client using an SSH backed connection.
type Client struct {
	*rpc.Client
	Config      *ssh.ClientConfig
	ChannelName string
	sshClient   *ssh.Client
}

// NewClient returns a new Client to handle RPC requests.
func NewClient() *Client {

	config := &ssh.ClientConfig{
		User: "sshrpc",
		Auth: []ssh.AuthMethod{
			ssh.Password("sshrpc"),
		},
	}

	return &Client{nil, config, DefaultRPCChannel, nil}

}

// Connect starts a client connection to the given SSH/RPC server.
func (c *Client) Connect(address string) {

	sshClient, err := ssh.Dial("tcp", address, c.Config)
	if err != nil {
		panic("Failed to dial: " + err.Error())
	}
	c.sshClient = sshClient

	// Each ClientConn can support multiple channels
	channel, err := c.openRPCChannel(c.ChannelName)
	if err != nil {
		panic("Failed to create channel: " + err.Error())
	}

	c.Client = rpc.NewClient(channel)

	return
}

// openRPCChannel opens an SSH RPC channel and makes an ssh subsystem request to trigger remote RPC server start
func (c *Client) openRPCChannel(channelName string) (ssh.Channel, error) {
	channel, in, err := c.sshClient.OpenChannel(channelName, nil)
	if err != nil {
		return nil, err
	}

	// SSH Documentation states that this go channel of requests needs to be serviced
	go func(reqs <-chan *ssh.Request) error {
		for msg := range reqs {
			switch msg.Type {

			default:
				if msg.WantReply {
					msg.Reply(false, nil)
				}
			}
		}
		return nil
	}(in)

	var msg struct {
		Subsystem string
	}
	msg.Subsystem = RPCSubsystem

	ok, err := channel.SendRequest("subsystem", true, ssh.Marshal(&msg))
	if err == nil && !ok {
		return nil, err
	}

	return channel, nil
}
