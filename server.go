package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type SocketServer interface {
	HandleWS(writer http.ResponseWriter, request *http.Request)
	EmitTo(event string, data string, ids []uuid.UUID)
	ExceptTo(event string, data string, exceptIds []uuid.UUID)
	Broadcast(event string, data string)
	AddSocket(s *Socket)
	RemoveSocket(s *Socket)
	Room(name string) *Room
	CreateRoom(roomName string) *Room
	DeleteRoom(roomName string)
	GetRoom(roomName string) *Room
	On(event string, callback SocketEventCallback)
	OnConnect(callback Callback)
	OnDisconnect(Callback Reason)
	CountSockets() uint64
	start()
	Close()
}

type Callback func(*Socket)
type Reason func(string)

func stringify(event string, data string) (string, error) {
	var socketMessage SocketMessage
	socketMessage.Event = event
	socketMessage.Payload = data
	message, err := json.Marshal(socketMessage)
	if err != nil {
		return "", err
	}
	return string(message), nil
}

var (
	OnConnectError = "OnConnect callback can be set only once"
	OnDisconnectError = "OnDisconnect callback can be set only once"
	
)

var socketServerInstance SocketServer

var upgrade = websocket.Upgrader{
	ReadBufferSize:  4092,
	WriteBufferSize: 4092,
}

type socketServer struct {
	rooms             sync.Map
	sockets           sync.Map
	counter 				 uint64
	connectedSockets  chan *Socket
	leavedSockets     chan *Socket
	socketChannel     chan *Socket
	events            sync.Map
	onConnect         Callback
	onConnectIsSet    bool
	onDisconnect      Reason
	onDisconnectIsSet bool
}

func (ss *socketServer) On(event string, callback SocketEventCallback) {
	_, ok := ss.events.Load(event)
	if !ok {
		ss.events.Store(event, callback)
	}
}

func (ss *socketServer) start() {
	for {
		select {
		case socket := <-ss.connectedSockets:
			if ss.onConnect != nil {
				ss.onConnect(socket)
			}
			ss.socketChannel <- socket
		case socket := <-ss.leavedSockets:
			if ss.onDisconnect != nil {
				ss.onDisconnect(socket.conn.Close().Error())
			}
		case socket := <-ss.socketChannel:
			go func() {
				ss.events.Range(func(key, value interface{}) bool {
					event := key.(string)
					callback := value.(SocketEventCallback)
					socket.On(event, callback)
					return true
				})
			}()
			socket.Listen()
		}
	}
}

func (ss *socketServer) HandleWS(writer http.ResponseWriter, request *http.Request)  {
	upgrade.CheckOrigin = func(r *http.Request) bool {
		return true
	}

	ws, err := upgrade.Upgrade(writer, request, nil)
	if err != nil {
		 panic(err)
	}
	s := NewSocket(ws, request, ss)
	ss.AddSocket(s)
	
}

func (ss *socketServer) AddSocket(s *Socket) {
	ss.sockets.Store(s.Id, s)
	ss.connectedSockets <- s
	atomic.AddUint64(&ss.counter, 1)
}

func (ss *socketServer) OnConnect(callback Callback) {
	if ss.onConnectIsSet {
		log.Panic(OnConnectError)
		return
	}
	ss.onConnect = callback
}

func (ss *socketServer) OnDisconnect(callback Reason) {
	if ss.onDisconnectIsSet {
		log.Panic(OnConnectError)
		return
	}
	ss.onDisconnect = callback
}

func (ss *socketServer) RemoveSocket(s *Socket) {
	ss.rooms.Range(func(key, value interface{}) bool {
		room := value.(*Room)
		room.Remove(s)
		return true
	})

	ss.sockets.Delete(s.Id)
	atomic.AddUint64(&ss.counter, ^uint64(0))
	ss.leavedSockets <- s
}

func (ss *socketServer) Room(name string) *Room {
	room := ss.GetRoom(name)
	if room == nil {
		panic("Room does not exist")
	}
	return room
}


func (ss *socketServer) EmitTo(event string, data string, ids []uuid.UUID) {
	ss.sockets.Range(func(key, value interface{}) bool {
		socket := value.(*Socket)
		for _, id := range ids {
			if socket.Id == id {
				socket.Send(event, data)
			}
		}
		return true
	})
}

func (ss *socketServer) ExceptTo(event string, data string, exceptIds []uuid.UUID) {
	ss.sockets.Range(func(key, value interface{}) bool {
		socket := value.(*Socket)
		for _, exceptId := range exceptIds {
			if socket.Id != exceptId {
				socket.Send(event, data)
			}
		}
		return true
	})
}


func (ss *socketServer) Broadcast(event string, data string) {

	ss.sockets.Range(func(key, value interface{}) bool {
		socket := value.(*Socket)
		socket.Send(event, data)
		return true
	})
}

func (ss *socketServer) CreateRoom(roomName string) *Room {
	if room := ss.GetRoom(roomName); room == nil {
		ss.rooms.Store(roomName, NewRoom(roomName))
	}
	room, _ := ss.rooms.Load(roomName)
	return room.(*Room)
}

func (ss *socketServer) DeleteRoom(roomName string) {
	ss.rooms.Delete(roomName)
}

func (ss *socketServer) GetRoom(roomName string) *Room {
	room, ok := ss.rooms.Load(roomName)
	if !ok {
		return nil
	}
	return room.(*Room)
}

func (ss *socketServer) CountSockets() uint64 {
	return atomic.LoadUint64(&ss.counter)
}

func (ss *socketServer) Close() {
	ss.sockets.Range(func(key, value interface{}) bool {
		socket := value.(*Socket)
		socket.Close()
		return true
	})
}

func GetInstance() SocketServer {
	if socketServerInstance == nil {
		socketServerInstance = NewSocketServer()
	}
	return socketServerInstance
}


func NewSocketServer() SocketServer {
		server := &socketServer{
			connectedSockets: make(chan *Socket, 100),
			leavedSockets:    make(chan *Socket, 100),
			socketChannel:    make(chan *Socket, 100),
		}

		go server.start()
		return server
	}
