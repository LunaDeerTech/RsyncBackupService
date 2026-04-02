package ws

import (
	"net/http"
	"sync"
	"time"

	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"github.com/LunaDeerTech/RsyncBackupService/internal/service"
	"github.com/gin-gonic/gin"
	xwebsocket "golang.org/x/net/websocket"
)

const (
	writeWait = 10 * time.Second
)

var bridgeNow = func() time.Time {
	return time.Now().UTC()
}

type Client struct {
	hub       *Hub
	conn      *xwebsocket.Conn
	send      chan service.ProgressEvent
	closeOnce sync.Once
}

func ServeProgress(hub *Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		if hub == nil {
			c.AbortWithStatusJSON(500, gin.H{"error": "progress hub unavailable"})
			return
		}

		server := xwebsocket.Server{
			Handshake: func(*xwebsocket.Config, *http.Request) error {
				return nil
			},
			Handler: xwebsocket.Handler(func(conn *xwebsocket.Conn) {
				client := &Client{
					hub:  hub,
					conn: conn,
					send: make(chan service.ProgressEvent, 16),
				}
				hub.Register(client)

				go client.writePump()
				client.readPump()
			}),
		}

		server.ServeHTTP(c.Writer, c.Request)
	}
}

func BridgeProgress(executorService *service.ExecutorService, hub *Hub) func() {
	if executorService == nil || hub == nil {
		return func() {}
	}
	var mu sync.Mutex
	lastBroadcastAt := make(map[string]time.Time)

	return executorService.SubscribeProgress(func(event service.ProgressEvent) {
		if shouldThrottleProgressEvent(event) {
			now := bridgeNow()
			mu.Lock()
			lastSentAt := lastBroadcastAt[event.TaskID]
			if !lastSentAt.IsZero() && now.Sub(lastSentAt) < time.Second {
				mu.Unlock()
				return
			}
			lastBroadcastAt[event.TaskID] = now
			mu.Unlock()
		} else {
			mu.Lock()
			delete(lastBroadcastAt, event.TaskID)
			mu.Unlock()
		}

		hub.Broadcast(event)
	})
}

func shouldThrottleProgressEvent(event service.ProgressEvent) bool {
	return event.TaskID != "" && event.Status == model.BackupStatusRunning
}

func (c *Client) closeSend() {
	if c == nil {
		return
	}

	c.closeOnce.Do(func() {
		close(c.send)
	})
}

func (c *Client) readPump() {
	if c == nil || c.conn == nil {
		return
	}
	defer func() {
		if c.hub != nil {
			c.hub.Unregister(c)
		}
		_ = c.conn.Close()
	}()

	for {
		var discard []byte
		if err := xwebsocket.Message.Receive(c.conn, &discard); err != nil {
			return
		}
	}
}

func (c *Client) writePump() {
	if c == nil || c.conn == nil {
		return
	}
	defer func() {
		_ = c.conn.Close()
	}()

	for {
		event, ok := <-c.send
		_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
		if !ok {
			return
		}
		if err := xwebsocket.JSON.Send(c.conn, event); err != nil {
			return
		}
	}
}
