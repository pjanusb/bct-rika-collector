package sshclient

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

type Client struct {
	client    *ssh.Client
	closeOnce sync.Once
	closed    chan struct{}
}

func Dial(user string, password string, address string) (*Client, error) {
	netConnection, err := net.DialTimeout("tcp", address, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("SSH TCP connection failed: %w", err)
	}

	if err := netConnection.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		netConnection.Close()
		return nil, fmt.Errorf("cannot set SSH connection deadline: %w", err)
	}

	sshConfig := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	connection, channels, requests, err := ssh.NewClientConn(netConnection, address, sshConfig)
	if err != nil {
		netConnection.Close()
		return nil, fmt.Errorf("SSH authentication failed: %w", err)
	}

	if err := netConnection.SetDeadline(time.Time{}); err != nil {
		connection.Close()
		return nil, fmt.Errorf("cannot clear SSH connection deadline: %w", err)
	}

	result := &Client{client: ssh.NewClient(connection, channels, requests), closed: make(chan struct{})}
	go result.keepAlive()
	return result, nil
}

func (c *Client) Close() error {
	var err error
	c.closeOnce.Do(func() {
		close(c.closed)
		err = c.client.Close()
	})
	return err
}

func (c *Client) CombinedOutput(command string, stdin []byte) ([]byte, error) {
	session, err := c.client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("cannot create SSH session: %w", err)
	}
	defer session.Close()

	if stdin != nil {
		session.Stdin = bytes.NewReader(stdin)
	}

	output, err := session.CombinedOutput(command)
	if err != nil {
		return output, fmt.Errorf("remote command failed: %w", err)
	}
	return output, nil
}

func (c *Client) Stream(command string) (io.Reader, func() error, func(), error) {
	session, err := c.client.NewSession()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("cannot create SSH session: %w", err)
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		session.Close()
		return nil, nil, nil, fmt.Errorf("cannot open SSH stdout pipe: %w", err)
	}

	var stderr bytes.Buffer
	session.Stderr = &stderr
	if err := session.Start(command); err != nil {
		session.Close()
		return nil, nil, nil, fmt.Errorf("cannot start remote command: %w", err)
	}

	wait := func() error {
		err := session.Wait()
		if err != nil {
			if stderr.Len() > 0 {
				return fmt.Errorf("remote command failed: %w: %s", err, bytes.TrimSpace(stderr.Bytes()))
			}
			return fmt.Errorf("remote command failed: %w", err)
		}
		return nil
	}
	closeSession := func() {
		session.Close()
	}
	return stdout, wait, closeSession, nil
}

func (c *Client) keepAlive() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	failures := 0
	for {
		select {
		case <-ticker.C:
			_, _, err := c.client.SendRequest("keepalive@openssh.com", true, nil)
			if err == nil {
				failures = 0
				continue
			}
			failures++
			if failures >= 2 {
				c.Close()
				return
			}
		case <-c.closed:
			return
		}
	}
}
