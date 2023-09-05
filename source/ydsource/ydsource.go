package tcpsource

import (
	"bufio"
	"log"
	"net"

	"go.einride.tech/can"
)

type ydSource struct {
	source      chan can.Frame
	addressPort string
	label       string
}

func (ys *ydSource) Source() chan can.Frame {
	return ys.source
}

func (ys *ydSource) AddressPort() string {
	return ys.addressPort
}

func (ys *ydSource) Label() string {
	return ys.label
}

func NewYdSource(addressPort string) (*ydSource, error) {

	ydSource := ydSource{}
	var err error

	ydSource.label = "YD-" + addressPort
	ydSource.source = make(chan can.Frame)

	tcpConnection, err := net.Dial("tcp", addressPort)
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		for {
			message, _ := bufio.NewReader(tcpConnection).ReadString('\n')
			frame, err := parse(message)
			if err != nil {
				log.Println(err, message)
			} else {
				ydSource.source <- frame
			}
		}
	}()

	return &ydSource, nil
}
