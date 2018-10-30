package redis

import (
	"fmt"
	"net"
)

// The package contains a basic client to connect to Redis over TCP
// It allows to send and receive object and execute basic commands needed to communicate and
// exchange data over RESP protocol

// Client class provides configuration of the client struct to connect to the Redis
type Client struct {
	// TCPIP is an IP to ocnnect to
	TCPIP string
	// TCPPort is a port value to connect to
	TCPPort string
	// conn represents the connection TCP object
	Conn *net.TCPConn
}

var addr *net.TCPAddr
var client *Client

// Connect connects to the specified IP and Port using the TCP connector
func Connect(host string, port string) (*Client, error) {
	if client == nil || client.TCPIP != host || client.TCPPort != port {
		var err error
		addr, err = net.ResolveTCPAddr("tcp", host+":"+port)
		if err != nil {
			return nil, fmt.Errorf("redis client: invalid address (%v:%v) specified for TCP connection: %v", host, port, err)
		}
	}

	conn, err := net.DialTCP("tcp", nil, addr)
	if err != nil {
		return nil, fmt.Errorf("redis client: error connecting to %v:%v over TCP: %v", host, port, err)
	}

	client = &Client{
		TCPIP:   host,
		TCPPort: port,
		Conn:    conn,
	}

	return client, nil
}

// Send sends the command to the Redis server and returns erro if the command fails
func (c *Client) Send(command string) error {
	bytesSent, err := c.Conn.Write([]byte(command))
	if err != nil {
		return fmt.Errorf("redis client: sending the command to the server %v:%v failed: %v", c.TCPIP, c.TCPPort, err)
	}
	if bytesSent != len(command) {
		return fmt.Errorf("redis client: number of bytes sent (%v) are not equal to the size of the actual command %v", bytesSent, len(command))
	}

	return err
}

// SendReceive allows to send the command to Redis server and receive a response
// the response []byte will be returned, or (nil, error) in case of an error
func (c *Client) SendReceive(command string) ([]byte, error) {
	err := c.Send(command)
	if err != nil {
		return nil, err
	}

	var data []byte
	size := 0
	for {
		buff := make([]byte, 32)
		bytesRead, err := c.Conn.Read(buff)
		if err != nil {
			return nil, err
		}

		size += bytesRead

		if bytesRead < 32 {
			data = append(data, buff[:bytesRead]...)
			break
		} else {
			data = append(data, buff...)
		}
	}
	return data[:size], nil
}
