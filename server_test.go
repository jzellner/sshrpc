package sshrpc

import (
	"fmt"
	"log"
	"sort"
	"testing"
	"time"

	"dev.justinjudd.org/justin/sshrpc/testdata"
	"golang.org/x/crypto/ssh"
)

type SimpleServer struct{}

func (s *SimpleServer) Hello(name *string, out *string) error {
	log.Println("Name: ", *name)
	*out = fmt.Sprintf("Hello %s", *name)
	return nil
}

func TestSimpleServer(t *testing.T) {
	s := NewServer()

	private, err := ssh.ParsePrivateKey(testdata.ServerRSAKey)
	if err != nil {
		log.Fatal("Failed to parse private key")
	}

	s.Config.AddHostKey(private)

	s.Register(new(SimpleServer))
	t.Log("preparing server")

	go s.StartServer("localhost:2022")

	time.Sleep(3 * time.Second)

	t.Log("preparing client")
	client := NewClient()
	client.Connect("localhost:2022")
	defer client.Close()
	var reply string
	err = client.Call("SimpleServer.Hello", "Test Name", &reply)
	if err != nil {
		t.Errorf("Unable to make rpc call: %s", err.Error())
	}
	log.Println("Reply: ", reply)
	if reply != "Hello Test Name" {
		t.Errorf("Simple Server Test Failed: Expected 'Hello Test Name', Recieved: '%s'", reply)
	}

}

type AdvancedServer struct{}

type AdvancedType struct {
	Ints []int
}

func (s *AdvancedServer) SortInts(req *AdvancedType, out *AdvancedType) error {
	sort.Ints(req.Ints)
	*out = *req
	return nil
}

func TestAdvancedServer(t *testing.T) {
	s := NewServer()
	s.Subsystem = "Advanced"

	private, err := ssh.ParsePrivateKey(testdata.ServerRSAKey)
	if err != nil {
		log.Fatal("Failed to parse private key")
	}

	s.Config.AddHostKey(private)

	s.Register(new(AdvancedServer))
	t.Log("preparing server")

	go s.StartServer("localhost:3022")

	time.Sleep(3 * time.Second)

	t.Log("preparing client")
	client := NewClient()
	client.Subsystem = "Advanced"
	client.Connect("localhost:3022")
	defer client.Close()
	var reply AdvancedType
	unsorted := AdvancedType{[]int{1, 3, 5, 7, 2, 4, 6}}
	err = client.Call("AdvancedServer.SortInts", unsorted, &reply)
	if err != nil {
		t.Errorf("Unable to make rpc call: %s", err.Error())
	}
	log.Println("Reply: ", reply)
	if !sort.IntsAreSorted(reply.Ints) {
		t.Errorf("Advanced Server Test Failed: Expected '{1,2,3,4,5,6,7}', Recieved: '%v'", reply)
	}

}
