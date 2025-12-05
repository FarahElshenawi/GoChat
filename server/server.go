package main

import (
	"fmt"
	"log"
	"net"
	"net/rpc"
	"sync"
	"time"
)

type Message struct {
	ID      int
	Sender  string
	Content string
	Time    time.Time
}

type Client struct {
	ID       string
	LastSeen int
	JoinedAt time.Time
	Original string // original requested name
}

type ChatRoom struct {
	mu        sync.RWMutex
	clients   map[string]*Client
	logs      []Message
	nextID    int
	nameUsage map[string]int
}

func NewChatRoom() *ChatRoom {
	return &ChatRoom{
		clients:   make(map[string]*Client),
		logs:      []Message{},
		nextID:    1,
		nameUsage: make(map[string]int),
	}
}

// ----------------------------
// RPC Argument Types
// ----------------------------

type JoinArgs struct {
	RequestedName string
}

type JoinReply struct {
	Success      bool
	AssignedName string
	Message      string
}

type SendArgs struct {
	ID      string
	Message string
}

type SendReply struct {
	Success bool
}

type UpdateArgs struct {
	ID        string
	LastMsgID int
}

type UpdateReply struct {
	Messages []Message
	NewMsgID int
}

// ----------------------------
// Helper: Create Unique Name
// ----------------------------

func (cr *ChatRoom) assignName(base string) string {
	if base == "" {
		base = "Guest"
	}

	if _, exists := cr.clients[base]; !exists {
		return base
	}

	for i := 1; i <= 99; i++ {
		candidate := fmt.Sprintf("%s%d", base, i)
		if _, exists := cr.clients[candidate]; !exists {
			return candidate
		}
	}

	return fmt.Sprintf("%s_%d", base, time.Now().UnixNano()%9999)
}

// ----------------------------
// RPC Methods
// ----------------------------

func (cr *ChatRoom) Join(args JoinArgs, reply *JoinReply) error {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	chosen := cr.assignName(args.RequestedName)

	cr.clients[chosen] = &Client{
		ID:       chosen,
		LastSeen: cr.nextID - 1,
		JoinedAt: time.Now(),
		Original: args.RequestedName,
	}

	sys := Message{
		ID:      cr.nextID,
		Sender:  "System",
		Content: fmt.Sprintf("User %s joined the chat", chosen),
		Time:    time.Now(),
	}
	cr.nextID++
	cr.logs = append(cr.logs, sys)

	reply.Success = true
	reply.AssignedName = chosen
	reply.Message = "Welcome! You are now " + chosen

	fmt.Printf("[JOIN] %s connected.\n", chosen)
	return nil
}

func (cr *ChatRoom) Send(args SendArgs, reply *SendReply) error {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	cl, ok := cr.clients[args.ID]
	if !ok {
		return fmt.Errorf("user not registered")
	}

	msg := Message{
		ID:      cr.nextID,
		Sender:  args.ID,
		Content: args.Message,
		Time:    time.Now(),
	}

	cr.logs = append(cr.logs, msg)
	cl.LastSeen = msg.ID
	cr.nextID++

	fmt.Printf("[MSG] %s → %s\n", args.ID, args.Message)

	reply.Success = true
	return nil
}

func (cr *ChatRoom) GetUpdates(args UpdateArgs, reply *UpdateReply) error {
	cr.mu.RLock()
	defer cr.mu.RUnlock()

	if _, ok := cr.clients[args.ID]; !ok {
		return fmt.Errorf("unknown client")
	}

	newList := []Message{}
	maxID := args.LastMsgID

	for _, m := range cr.logs {
		if m.ID > args.LastMsgID {
			if m.Sender == args.ID { // no echo
				continue
			}
			newList = append(newList, m)
			maxID = m.ID
		}
	}

	reply.Messages = newList
	reply.NewMsgID = maxID

	return nil
}

func (cr *ChatRoom) Leave(args struct{ ID string }, reply *JoinReply) error {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	_, exists := cr.clients[args.ID]
	if !exists {
		return nil
	}

	delete(cr.clients, args.ID)

	leaveMsg := Message{
		ID:      cr.nextID,
		Sender:  "System",
		Content: fmt.Sprintf("User %s left the chat", args.ID),
		Time:    time.Now(),
	}
	cr.nextID++
	cr.logs = append(cr.logs, leaveMsg)

	fmt.Printf("[LEAVE] %s disconnected.\n", args.ID)

	reply.Success = true
	reply.Message = "Disconnected"
	return nil
}

// ----------------------------
// Server
// ----------------------------

func main() {
	room := NewChatRoom()

	rpc.Register(room)

	listener, err := net.Listen("tcp", "127.0.0.1:1234")
	if err != nil {
		log.Fatalf("Could not start server: %v", err)
	}

	fmt.Println("Chat server running on port 1234...")
	fmt.Println("Waiting for users...")

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Accept error:", err)
			continue
		}
		go rpc.ServeConn(conn)
	}
}
