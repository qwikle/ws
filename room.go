package main

import (
	"sync"
	"sync/atomic"

	"github.com/google/uuid"
)

type Room struct {
	Name    string
	Sockets sync.Map
	count   uint64
}

func (r *Room) Add(s *Socket) {
	if r.GetSocket(s.Id) != nil {
		return
	}
	r.Sockets.Store(s.Id, s)
	atomic.AddUint64(&r.count, 1)
}

func (r *Room) Remove(s *Socket) {
	r.Sockets.Delete(s.Id)
	atomic.AddUint64(&r.count, ^uint64(0))
}

func (r *Room) GetSocket(id uuid.UUID) *Socket {
	if socket, ok := r.Sockets.Load(id); ok {
		return socket.(*Socket)
	}
	return nil
}

func (r *Room) Broadcast(event string, data string) *Room {
	r.Sockets.Range(func(key, value interface{}) bool {
		socket := value.(*Socket)
		socket.Send(event, data)
		return true
	})
	return r
}

func (r *Room) Emit(event string, data string, except *Socket) *Room {
	r.Sockets.Range(func(key, value interface{}) bool {
		socket := value.(*Socket)
		if socket.Id != except.Id {
			socket.Send(event, data)
		}
		return true
	})
	return r
}

func (r *Room) EmitTo(event string, data string, ids []uuid.UUID) *Room {
	r.Sockets.Range(func(key, value interface{}) bool {
		socket := value.(*Socket)
		for _, id := range ids {
			if socket.Id == id {
				socket.Send(event, data)
			}
		}
		return true
	})
	return r
}

func (r *Room) ExceptTo(event string, data string, exceptIds []uuid.UUID) *Room {
	r.Sockets.Range(func(key, value interface{}) bool {
		socket := value.(*Socket)
		for _, exceptId := range exceptIds {
			if socket.Id != exceptId {
				socket.Send(event, data)
			}
		}
		return true
	})
	return r
}

func NewRoom(name string) *Room {
	if name == "" {
		panic("Room name must be provided")
	}
	return &Room{
		Name: name,
	}
}

func (r *Room) Length() uint64 {
	return atomic.LoadUint64(&r.count)
}
