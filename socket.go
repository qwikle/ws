package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type SocketEventCallback func(data string, socket *Socket)

type SocketHandler struct {
	event    string
	callback SocketEventCallback
}

type Socket struct {
	conn            *websocket.Conn
	Id              uuid.UUID
	ip              string
	mu              sync.Mutex
	IncomingMessage chan string
	OutgoingMessage chan string
	events          map[string]SocketHandler
	Payload         map[string]interface{}
	headers         http.Header
	room            *Room
	server          *socketServer
}

func (s *Socket) SetData(key string, value interface{}) {
	s.Payload[key] = value
}

func (s *Socket) GetData(key string) interface{} {
	return s.Payload[key]
}

func (s *Socket) Send(event string, data string) {
	message, err := stringify(event, data)
	if err != nil {
		fmt.Println(err)
		return
	}
	s.OutgoingMessage <- message
}

func (s *Socket) listenOutgoingMessage() {
	for {
		message, ok := <-s.OutgoingMessage
		if !ok {
			return
		}

		err := s.conn.WriteMessage(websocket.TextMessage, []byte(message))
		if err != nil {
			fmt.Println("Error while sending message")
			fmt.Println(err)
			return
		}
	}
}

func (s *Socket) listenIncomingMessage() {

	for {
		message, ok := <-s.IncomingMessage
		if !ok {
			return
		}
		var socketMessage SocketMessage
		err := json.Unmarshal([]byte(message), &socketMessage)
		if err != nil {
			fmt.Println(err)
			continue
		}
		if handler, exists := s.events[socketMessage.Event]; exists {
			handler.callback(socketMessage.Payload, s)
		}
	}
}


func (s *Socket) readMessage() {
	for {
		var socketMessage SocketMessage
		err := s.conn.ReadJSON(&socketMessage)
		if err != nil {
			return
		}
		if _, exists := s.events[socketMessage.Event]; exists {
			message, err := json.Marshal(socketMessage)
			if err != nil {
				fmt.Println(err)
				continue
			}
			s.IncomingMessage <- string(message)
		}
	}
}

func (s *Socket) Listen() {
	go s.listenOutgoingMessage()
	go s.listenIncomingMessage()
	go s.readMessage()
}

type SocketMessage struct {
	Event   string `json:"event"`
	Payload string `json:"payload"`
}

func (s *Socket) Join(room string) *Room {
	s.server.Room(room).Add(s)
	s.room = s.server.GetRoom(room)
	return s.room
}

func (s *Socket) JoinOrCreate(room string) *Room {
	existingRoom := s.server.GetRoom(room)
	if existingRoom == nil {
		existingRoom = s.server.CreateRoom(room)
	}
	s.room = existingRoom
	s.room.Add(s)
	return s.room
}


func (s *Socket) LeaveAll() {
	s.server.rooms.Range(func(key, value interface{}) bool {
		room := value.(*Room)
		room.Remove(s)
		return true
	})
}




func (s *Socket) Close() {
	time.AfterFunc(1*time.Millisecond, func() {
	s.server.RemoveSocket(s)
	close(s.IncomingMessage)
	close(s.OutgoingMessage)
	err := s.conn.Close()
	if err != nil {
		panic(err)
	}
	})
}

func (s *Socket) On(event string, callback SocketEventCallback) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events[event] = SocketHandler{
		event:    event,
		callback: callback,
	}
}


func (s *Socket) GetHeader(key string) string {
	return s.headers.Get(key)
}

func (s *Socket) Headers() http.Header {
	return s.headers
}

func (s *Socket) GetIP() string {
	return s.ip
}

func (s *Socket) Room(name string) *Room {
	s.room = s.server.GetRoom(name)
	if s.room == nil {
		panic("Room does not exist")
	}
	socket := s.room.GetSocket(s.Id)
	if socket == nil {
		panic("Socket is not allowed to emit to this room")
	}
	return s.room
}

func NewSocket(conn *websocket.Conn, r *http.Request, server *socketServer) *Socket {
	return &Socket{
		conn:            conn,
		ip:              r.RemoteAddr,
		Id:              uuid.Must(uuid.NewRandom()),
		IncomingMessage: make(chan string, 100),
		OutgoingMessage: make(chan string, 100),
		events:          make(map[string]SocketHandler),
		Payload:         make(map[string]interface{}),
		headers:         r.Header,
		server:          server,
	}
}
