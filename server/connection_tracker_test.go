package server

import (
	"net"
	"testing"
	"time"
)

func TestConnectionTrackerClosesActiveConnections(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	tracker := NewConnectionTracker()
	trackedListener := tracker.Wrap(listener)
	accepted := make(chan net.Conn, 1)
	go func() {
		connection, acceptErr := trackedListener.Accept()
		if acceptErr == nil {
			accepted <- connection
		}
	}()

	client, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	serverConnection := <-accepted
	defer serverConnection.Close()

	tracker.CloseAll()
	if err = client.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err = client.Read(make([]byte, 1)); err == nil {
		t.Fatal("client connection remained open after tracker drain")
	}
}

func TestClosedConnectionTrackerRejectsNewConnections(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	tracker := NewConnectionTracker()
	trackedListener := tracker.Wrap(listener)
	tracker.CloseAll()
	acceptErr := make(chan error, 1)
	go func() {
		_, err := trackedListener.Accept()
		acceptErr <- err
	}()

	client, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	select {
	case err = <-acceptErr:
		if err == nil {
			t.Fatal("closed tracker accepted a new connection")
		}
	case <-time.After(time.Second):
		t.Fatal("accept did not return after the tracker rejected a connection")
	}
}
