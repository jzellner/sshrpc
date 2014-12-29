package sshrpc

import (
	"fmt"
	"log"
	"net"
	"net/rpc"

	"golang.org/x/crypto/ssh"
)

// Server represents an SSH Server that spins up RPC servers when requested.
type Server struct {
	*rpc.Server
	Config    *ssh.ServerConfig
	Subsystem string
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
	return &Server{rpc.NewServer(), c, "sshrpc"}

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
		go s.handleChannels(chans)
	}
}

func (s *Server) handleRequests(reqs <-chan *ssh.Request) {
	for req := range reqs {
		log.Printf("recieved out-of-band request: %+v", req)
	}
}

func (s *Server) handleChannels(chans <-chan ssh.NewChannel) {
	// Service the incoming Channel channel.
	for newChannel := range chans {
		// Channels have a type, depending on the application level
		// protocol intended. In the case of a shell, the type is
		// "session" and ServerShell may be used to present a simple
		// terminal interface.
		if t := newChannel.ChannelType(); t != "session" {
			newChannel.Reject(ssh.UnknownChannelType, fmt.Sprintf("unknown channel type: %s", t))
			continue
		}
		channel, requests, err := newChannel.Accept()
		if err != nil {
			log.Printf("could not accept channel (%s)", err)
			continue
		}

		// Sessions have out-of-band requests such as "shell", "pty-req" and "env"
		go func(in <-chan *ssh.Request) {
			for req := range in {
				ok := false
				switch req.Type {

				case "subsystem":
					ok = true
					log.Printf("subsystem '%s'", req.Payload)
					switch string(req.Payload[4:]) {
					case s.Subsystem:
						go s.ServeConn(channel)
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
}
