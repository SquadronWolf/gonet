package udp

import (
	"errors"
	"fmt"
	"network/ipv4"

	"github.com/hsheth2/logs"
)

const MAX_UDP_PACKET_LEN = 65507

type UDP_Read_Manager struct {
	reader *ipv4.IP_Reader
	buff   map[uint16](map[ipv4.IPaddress](chan []byte))
}

type UDP_Reader struct {
	manager   *UDP_Read_Manager
	bytes     <-chan []byte
	port      uint16 // ports
	ipAddress ipv4.IPaddress
}

func NewUDP_Read_Manager() (*UDP_Read_Manager, error) {
	irm := ipv4.GlobalIPReadManager

	ipr, err := ipv4.NewIP_Reader(irm, "*", ipv4.UDP_PROTO)
	if err != nil {
		return nil, err
	}

	x := &UDP_Read_Manager{
		reader: ipr,
		buff:   make(map[uint16](map[ipv4.IPaddress](chan []byte))),
	}

	go x.readAll()

	return x, nil
}

func (x *UDP_Read_Manager) readAll() {
	for {
		rip, lip, _, payload, err := x.reader.ReadFrom()
		if err != nil {
			logs.Error.Println(err)
			continue
		}
		//fmt.Println(b)
		//fmt.Println("UDP header and payload: ", payload)

		dst := (((uint16)(payload[2])) * 256) + ((uint16)(payload[3]))
		//fmt.Println(dst)

		if len(payload) < UDP_HEADER_SZ {
			logs.Info.Println("Dropping Small UDP packet:", payload)
			continue
		}

		headerLen := uint16(payload[4])<<8 | uint16(payload[5])
		if !ipv4.VerifyTransportChecksum(payload[:UDP_HEADER_SZ], rip, lip, headerLen, ipv4.UDP_PROTO) {
			logs.Info.Println("Dropping UDP Packet for bad checksum:", payload)
			continue
		}

		payload = payload[UDP_HEADER_SZ:]
		//fmt.Println(payload)

		portBuf, ok := x.buff[dst]
		//fmt.Println(ok)
		if ok {
			if c, ok := portBuf[rip]; ok {
				//fmt.Println("Found exact IP match for port", dst)
				go func() { c <- payload }()
			} else if c, ok := portBuf["*"]; ok {
				//fmt.Println("Found default IP match for port", dst)
				go func() { c <- payload }()
			}
		} else {
			//logs.Info.Println("Dropping UDP packet:", payload)
		}
	}
}

func (x *UDP_Read_Manager) NewUDP(port uint16, ip ipv4.IPaddress) (*UDP_Reader, error) {
	// add the port if not already there
	if _, found := x.buff[port]; !found {
		x.buff[port] = make(map[ipv4.IPaddress](chan []byte))
	}

	// add the ip to the port's list
	if _, found := x.buff[port][ip]; !found {
		x.buff[port][ip] = make(chan []byte)
		return &UDP_Reader{port: port, bytes: x.buff[port][ip], manager: x, ipAddress: ip}, nil
	} else {
		return nil, errors.New("Another application is already listening to port " + fmt.Sprintf("%v", port) + " with IP " + string(ip))
	}
}

func (c *UDP_Reader) Read(size int) ([]byte, error) {
	data := <-c.bytes
	if len(data) > size {
		data = data[:size]
	}
	return data, nil
}

func (c *UDP_Reader) Close() error {
	delete(c.manager.buff, c.port)
	return nil
}
