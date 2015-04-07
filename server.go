package sshrpc

import (
	"fmt"
	"log"
	"net"
	"net/rpc"

	"golang.org/x/crypto/ssh"
)

// DefaultRPCChannel is the Channel Name that will be used to carry the RPC traffic, can be changed in Serverr and Client
const DefaultRPCChannel = "RPCChannel"

// RPCSubsystem is the subsystem that will be used to trigger RPC endpoint creation
const RPCSubsystem = "RPCSubsystem"

// CallbackFunc to be called when reverse RPC client is created
type CallbackFunc func(RPCClient *rpc.Client)

// Server represents an SSH Server that spins up RPC servers when requested.
type Server struct {
	*rpc.Server
	Config      *ssh.ServerConfig
	ChannelName string

	CallbackFunc
}

// NewServer returns a new Server to handle incoming SSH and RPC requests.
func NewServer() *Server {
	c := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			if c.User() == "sshrpc" && string(pass) == "sshrpc" {
				return nil, nil
			}
			return nil, fmt.Errorf("password rejected for %q", c.User())
		},
	}
	return &Server{Server: rpc.NewServer(), Config: c, ChannelName: DefaultRPCChannel}

}

// StartServer starts the server listening for requests
func (s *Server) StartServer(address string) {

	// Once a ServerConfig has been configured, connections can be accepted.
	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatal("failed to listen on ", address)
	}

	// Accept all connections
	log.Print("listening on ", address)
	for {
		tcpConn, err := listener.Accept()
		if err != nil {
			log.Printf("failed to accept incoming connection (%s)", err)
			continue
		}
		// Before use, a handshake must be performed on the incoming net.Conn.
		sshConn, chans, reqs, err := ssh.NewServerConn(tcpConn, s.Config)

		if err != nil {
			log.Printf("failed to handshake (%s)", err)
			continue
		}

		log.Printf("new ssh connection from %s (%s)", sshConn.RemoteAddr(), sshConn.ClientVersion())
		// Print incoming out-of-band Requests
		go s.handleRequests(reqs)
		// Accept all channels
		go s.handleChannels(chans, sshConn)

	}
}

// handleRequests handles global out-of-band SSH Requests
func (s *Server) handleRequests(reqs <-chan *ssh.Request) {
	for req := range reqs {
		log.Printf("recieved out-of-band request: %+v", req)
	}
}

// handleChannels handels SSH Channel requests and their local out-of-band SSH Requests
func (s *Server) handleChannels(chans <-chan ssh.NewChannel, sshConn ssh.Conn) {
	// Service the incoming Channel channel.
	for newChannel := range chans {

		log.Printf("Received channel: %v", newChannel.ChannelType())
		// Check the type of channel
		if t := newChannel.ChannelType(); t != s.ChannelName {
			newChannel.Reject(ssh.UnknownChannelType, fmt.Sprintf("unknown channel type: %s", t))
			continue
		}
		channel, requests, err := newChannel.Accept()
		if err != nil {
			log.Printf("could not accept channel (%s)", err)
			continue
		}
		log.Printf("Accepted channel")

		// Channels can have out-of-band requests
		go func(in <-chan *ssh.Request) {
			for req := range in {
				ok := false
				switch req.Type {

				case "subsystem":
					ok = true
					log.Printf("subsystem '%s'", req.Payload[4:])
					switch string(req.Payload[4:]) {
					//RPCSubsystem Request made indicates client desires RPC Server access
					case RPCSubsystem:
						go s.ServeConn(channel)
						log.Printf("Started SSH RPC")
						// triggers reverse RPC connection as well
						clientChannel, err := openRPCClientChannel(sshConn, s.ChannelName+"-reverse")
						if err != nil {
							log.Printf("Failed to create client channel: " + err.Error())
							continue
						}
						rpcClient := rpc.NewClient(clientChannel)
						if s.CallbackFunc != nil {
							s.CallbackFunc(rpcClient)
						}
						log.Printf("Started SSH RPC client")
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
}
