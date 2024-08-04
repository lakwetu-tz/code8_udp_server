package main

import (
	"fmt"
	"log"
	"net"

	"github.com/filipkroca/teltonikaparser"
)

//Server is exported and hold the port number
type Server struct {
	Protocol string
	IP       []byte
	Port     int
}

//New start listening on specified port, should provide callback function. On new connection invoke a callback function as a new goroutine
func (t *Server) New(callBack func(udpc *net.UDPConn, buf *[]byte, len int, addr *net.UDPAddr)) {

	udpc, err := net.ListenUDP(t.Protocol, &net.UDPAddr{IP: t.IP, Port: t.Port, Zone: ""})
	if err != nil {
		fmt.Println(err)
		return
	}
	defer udpc.Close()

	fmt.Printf("Listening on %v\n", udpc.LocalAddr())

	for {
		//make a buffer
		buf := make([]byte, 4096)
		n, addr, err := udpc.ReadFromUDP(buf)
		if err != nil {
			log.Panic("error when listening ", err)
		}

		//slice data
		sliced := buf[:n]

		fmt.Printf("New connection from %v , fired a new goroutine \n", addr)
		//on connection fire new goroutine
		go callBack(udpc, &sliced, n, addr)
	}
}

func main() {

	server := Server{
		Protocol: "udp",
		IP:       []byte{0, 0, 0, 0},
		Port:     8800,
	}

	//create new server
	server.New(onUDPMessage)
	defer fmt.Println("server closed")

}

//onUDPMessage is invoked when packet arrive
func onUDPMessage(udpc *net.UDPConn, dataBs *[]byte, len int, addr *net.UDPAddr) {
	//conn := *udpc

	x, err := teltonikaparser.Decode(dataBs)
	if err != nil {
		log.Panic("Unable to decode packet", err)
	}

	fmt.Printf("%+v", x)

	(*udpc).WriteToUDP([]byte("hello world"), addr)

}