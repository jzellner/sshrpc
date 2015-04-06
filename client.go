package sshrpc

import (
	"log"
	"net/rpc"

	"golang.org/x/crypto/ssh"
)

// Client represents an RPC client using an SSH backed connection.
type Client struct {
	*rpc.Client
	Config      *ssh.ClientConfig
	ChannelName string
	sshClient   *ssh.Client
	RPCServer   *rpc.Server
}

// NewClient returns a new Client to handle RPC requests.
func NewClient() *Client {

	config := &ssh.ClientConfig{
		User: "sshrpc",
		Auth: []ssh.AuthMethod{
			ssh.Password("sshrpc"),
		},
	}

	return &Client{nil, config, DefaultRPCChannel, nil, rpc.NewServer()}

}

// Connect starts a client connection to the given SSH/RPC server.
func (c *Client) Connect(address string) {

	sshClient, err := ssh.Dial("tcp", address, c.Config)
	if err != nil {
		panic("Failed to dial: " + err.Error())
	}
	c.sshClient = sshClient

	c.openRPCServerChannel(c.ChannelName + "-reverse")

	// Each ClientConn can support multiple channels
	channel, err := openRPCClientChannel(c.sshClient.Conn, c.ChannelName)
	if err != nil {
		panic("Failed to create channel: " + err.Error())
	}

	c.Client = rpc.NewClient(channel)

}

// openRPCClientChannel opens an SSH RPC channel and makes an ssh subsystem request to trigger remote RPC server start
func openRPCClientChannel(conn ssh.Conn, channelName string) (ssh.Channel, error) {
	channel, in, err := conn.OpenChannel(channelName, nil)
	if err != nil {
		return nil, err
	}

	// SSH Documentation states that this go channel of requests needs to be serviced
	go ssh.DiscardRequests(in)

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

func (c *Client) openRPCServerChannel(channelName string) error {
	subChannel := c.sshClient.HandleChannelOpen(channelName)
	go func(chans <-chan ssh.NewChannel) {

		for newChannel := range chans {

			/* Don't need to check, already know what channel type is coming in
			// Check the type of channel
			if t := newChannel.ChannelType(); t != channelName {
				newChannel.Reject(ssh.UnknownChannelType, fmt.Sprintf("unknown channel type: %s", t))
				continue
			}
			*/
			acceptRPCServerRequest(c.RPCServer, newChannel)
		}
	}(subChannel)
	return nil
}

func acceptRPCServerRequest(rpcServer *rpc.Server, newChannel ssh.NewChannel) {
	channel, requests, err := newChannel.Accept()
	if err != nil {
		log.Printf("could not accept channel (%s)", err)
		return
	}
	log.Printf("Accepted channel")

	// Channels can have out-of-band requests
	go func(in <-chan *ssh.Request) {
		for req := range in {
			ok := false
			switch req.Type {

			case "subsystem":
				ok = true
				log.Printf("subsystem '%s'", req.Payload)
				switch string(req.Payload[4:]) {
				//RPCSubsystem Request made indicates client desires RPC Server access
				case RPCSubsystem:
					go rpcServer.ServeConn(channel)
					log.Printf("Started SSH RPC")
				default:
					log.Printf("Unknown subsystem: %s", req.Payload)
				}

			}
			if !ok {
				log.Printf("declining %s request...", req.Type)
			}
			req.Reply(ok, nil)
		}

	}(requests)
}

// Wait allows clients also acting as an RPC server to detect when the ssh connection ends
func (c *Client) Wait() error {
	return c.sshClient.Conn.Wait()
}
