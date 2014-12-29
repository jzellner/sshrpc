package sshrpc

import (
	"fmt"
	"io/ioutil"
	"log"

	"golang.org/x/crypto/ssh"
)

type ExampleServer struct{}

func (s *ExampleServer) Hello(name *string, out *string) error {
	*out = fmt.Sprintf("Hello %s", *name)
	return nil
}

func ExampleServer_StartServer() {

	s := NewServer()
	privateBytes, err := ioutil.ReadFile("id_rsa")
	if err != nil {
		log.Fatal("Failed to load private key (./id_rsa)")
	}

	private, err := ssh.ParsePrivateKey(privateBytes)
	if err != nil {
		log.Fatal("Failed to parse private key")
	}

	s.Config.AddHostKey(private)
	s.Register(new(ExampleServer))
	s.StartServer("localhost:2022")
}

func ExampleClient_Connect() {

	client := NewClient()
	client.Connect("localhost:2022")
	defer client.Close()
	var reply string
	err := client.Call("ExampleServer.Hello", "Example Name", &reply)
	if err != nil {
		//Handle Error
	}
	fmt.Println(reply)
}
