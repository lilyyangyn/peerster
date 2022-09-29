package udp

import (
	"errors"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.dedis.ch/cs438/transport"
	"golang.org/x/xerrors"
)

const bufSize = 65000

// NewUDP returns a new udp transport implementation.
func NewUDP() transport.Transport {
	return &UDP{}
}

// UDP implements a transport layer using UDP
//
// - implements transport.Transport
type UDP struct {
}

func checkValidAddr(address string) bool {
	chunks := strings.Split(address, ":")
	if len(chunks) != 2 {
		return false
	}
	if net.ParseIP(chunks[0]) == nil {
		return false
	}
	port, err := strconv.Atoi(chunks[1])
	if err != nil {
		return false
	}
	return port <= 65535
}

// CreateSocket implements transport.Transport
func (n *UDP) CreateSocket(address string) (transport.ClosableSocket, error) {
	if !checkValidAddr(address) {
		return nil, xerrors.Errorf("Invalid address %s", address)
	}

	udpAddr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, err
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return nil, err
	}
	address = conn.LocalAddr().String()

	return &Socket{
		conn:   conn,
		myAddr: address,
		ins:    packets{},
		outs:   packets{},
	}, nil
}

// Socket implements a network socket using UDP.
//
// - implements transport.Socket
// - implements transport.ClosableSocket
type Socket struct {
	conn   *net.UDPConn
	myAddr string
	ins    packets
	outs   packets
}

// Close implements transport.Socket. It returns an error if already closed.
func (s *Socket) Close() error {
	if s.conn == nil {
		return xerrors.Errorf("Socket already closed.")
	}
	s.conn.Close()
	s.conn = nil
	return nil
}

// Send implements transport.Socket
func (s *Socket) Send(dest string, pkt transport.Packet, timeout time.Duration) error {
	if !checkValidAddr(dest) {
		return xerrors.Errorf("Invalid address %s", dest)
	}
	destUdpAddr, err := net.ResolveUDPAddr("udp", dest)
	if err != nil {
		return err
	}

	if timeout != 0 {
		s.conn.SetWriteDeadline(time.Now().Add(timeout))
	} else {
		s.conn.SetWriteDeadline(time.Time{})
	}

	bytes, err := pkt.Marshal()
	if err != nil {
		return err
	}
	_, err = s.conn.WriteToUDP(bytes, destUdpAddr)
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return transport.TimeoutError(timeout)
	}
	if err != nil {
		return err
	}
	s.outs.add(pkt)
	return nil
}

// Recv implements transport.Socket. It blocks until a packet is received, or
// the timeout is reached. In the case the timeout is reached, return a
// TimeoutErr.
func (s *Socket) Recv(timeout time.Duration) (transport.Packet, error) {
	pkt := transport.Packet{}

	if timeout != 0 {
		s.conn.SetReadDeadline(time.Now().Add(timeout))
	} else {
		s.conn.SetReadDeadline(time.Time{})
	}

	buffer := make([]byte, bufSize)
	size, _, err := s.conn.ReadFromUDP(buffer)
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return pkt, transport.TimeoutError(timeout)
	}
	if err != nil {
		return pkt, err
	}
	err = pkt.Unmarshal(buffer[:size])
	if err != nil {
		return pkt, err
	}
	s.ins.add(pkt)
	return pkt, nil
}

// GetAddress implements transport.Socket. It returns the address assigned. Can
// be useful in the case one provided a :0 address, which makes the system use a
// random free port.
func (s *Socket) GetAddress() string {
	return s.myAddr
}

// GetIns implements transport.Socket
func (s *Socket) GetIns() []transport.Packet {
	return s.ins.getAll()
}

// GetOuts implements transport.Socket
func (s *Socket) GetOuts() []transport.Packet {
	return s.outs.getAll()
}

type packets struct {
	sync.Mutex
	data []transport.Packet
}

func (p *packets) add(pkt transport.Packet) {
	p.Lock()
	defer p.Unlock()

	p.data = append(p.data, pkt.Copy())
}

func (p *packets) getAll() []transport.Packet {
	p.Lock()
	defer p.Unlock()

	res := make([]transport.Packet, len(p.data))

	for i, pkt := range p.data {
		res[i] = pkt.Copy()
	}

	return res
}
