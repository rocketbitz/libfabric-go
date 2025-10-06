// Package main demonstrates a minimal libfabric client send/receive example.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/rocketbitz/libfabric-go/client"
	fi "github.com/rocketbitz/libfabric-go/fi"
)

func main() {
	cfg := client.Config{Timeout: 2 * time.Second, EndpointType: fi.EndpointTypeRDM}

	sender, err := client.Dial(cfg)
	if err != nil {
		log.Fatalf("dial sender: %v", err)
	}
	defer func() {
		_ = sender.Close()
	}()

	receiver, err := client.Dial(cfg)
	if err != nil {
		log.Fatalf("dial receiver: %v", err)
	}
	defer func() {
		_ = receiver.Close()
	}()

	receiverAddr, err := receiver.LocalAddress()
	if err != nil {
		log.Fatalf("receiver LocalAddress: %v", err)
	}
	receiverDest, err := sender.RegisterPeer(receiverAddr, false)
	if err != nil {
		log.Fatalf("register receiver peer: %v", err)
	}

	senderAddr, err := sender.LocalAddress()
	if err != nil {
		log.Fatalf("sender LocalAddress: %v", err)
	}
	senderDest, err := receiver.RegisterPeer(senderAddr, false)
	if err != nil {
		log.Fatalf("register sender peer: %v", err)
	}

	ctx := context.Background()

	go func() {
		buf := make([]byte, 256)
		n, addr, err := receiver.ReceiveFrom(ctx, buf)
		if err != nil {
			log.Fatalf("receive-from: %v", err)
		}
		fmt.Printf("receiver got %q from %v\n", string(buf[:n]), addr)

		ack := []byte("ack from receiver")
		if err := receiver.SendTo(ctx, senderDest, ack); err != nil {
			log.Fatalf("send ack: %v", err)
		}
	}()

	time.Sleep(50 * time.Millisecond)

	message := []byte("hello via SendTo/ReceiveFrom")
	if err := sender.SendTo(ctx, receiverDest, message); err != nil {
		log.Fatalf("send message: %v", err)
	}

	ackBuf := make([]byte, 256)
	n, addr, err := sender.ReceiveFrom(ctx, ackBuf)
	if err != nil {
		log.Fatalf("receive ack: %v", err)
	}
	fmt.Printf("sender got %q from %v\n", string(ackBuf[:n]), addr)
}
