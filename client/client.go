package main

import (
	"bufio"
	"fmt"
	"log"
	"net/rpc"
	"os"
	"strings"
	"time"
)

func main() {
	// Connect to the RPC server
	conn, err := rpc.Dial("tcp", "127.0.0.1:1234")
	if err != nil {
		log.Fatalf("Could not connect to server: %v", err)
	}
	defer conn.Close()

	input := bufio.NewReader(os.Stdin)

	// Ask user for a preferred name
	fmt.Print("Choose a username (leave blank for default): ")
	nameInput, _ := input.ReadString('\n')
	nameInput = strings.TrimSpace(nameInput)

	// Join ChatRoom
	var joinResp struct {
		Success      bool
		AssignedName string
		Message      string
	}

	err = conn.Call("ChatRoom.Join",
		struct{ RequestedName string }{RequestedName: nameInput},
		&joinResp,
	)
	if err != nil || !joinResp.Success {
		log.Fatalf("Join failed: %v %s", err, joinResp.Message)
	}

	username := joinResp.AssignedName
	fmt.Printf("\n%s\n\n", joinResp.Message)
	fmt.Println("Type messages and press Enter.")
	fmt.Println("Use 'exit' to leave the chat.")

	// For polling new updates
	lastSeen := 0
	recvStop := make(chan bool)

	// Background receiver to continuously fetch updates
	go func() {
		for {
			select {
			case <-recvStop:
				return
			case <-time.After(250 * time.Millisecond):
				var updateResp struct {
					Messages []struct {
						ID      int
						Sender  string
						Content string
					}
					NewMsgID int
				}

				err := conn.Call("ChatRoom.GetUpdates",
					struct {
						ID        string
						LastMsgID int
					}{
						ID:        username,
						LastMsgID: lastSeen,
					},
					&updateResp,
				)

				if err != nil {
					fmt.Println("\n[Connection lost]")
					recvStop <- true
					return
				}

				if len(updateResp.Messages) > 0 {
					for _, m := range updateResp.Messages {
						if m.Sender == "System" {
							fmt.Printf("\n[SYSTEM] %s\n", m.Content)
						} else {
							fmt.Printf("\n%s: %s\n", m.Sender, m.Content)
						}
						fmt.Print("> ")
					}
					lastSeen = updateResp.NewMsgID
				}
			}
		}
	}()

	// Leave function runs on exit
	defer func() {
		recvStop <- true
		var leaveResp struct {
			Success bool
			Message string
		}
		conn.Call("ChatRoom.Leave", struct{ ID string }{ID: username}, &leaveResp)
	}()

	// Main chat loop
	for {
		fmt.Print("> ")
		msg, _ := input.ReadString('\n')
		msg = strings.TrimSpace(msg)

		if strings.ToLower(msg) == "exit" {
			fmt.Println("Leaving chat...")
			break
		}

		if msg == "" {
			continue
		}

		var sendResp struct{ Success bool }
		err := conn.Call("ChatRoom.Send",
			struct {
				ID      string
				Message string
			}{
				ID:      username,
				Message: msg,
			},
			&sendResp)

		if err != nil {
			fmt.Printf("\n[Send error] %v\n", err)
			break
		}

		fmt.Printf("\n[You] %s\n", msg)
	}
}
