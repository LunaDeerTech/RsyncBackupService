package ws

import (
	"net/http"
	"sync"
	"time"

	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"github.com/LunaDeerTech/RsyncBackupService/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(*http.Request) bool {
		return true
	},
}

var bridgeNow = func() time.Time {
	return time.Now().UTC()
}

type Client struct {
	hub       *Hub
	conn      *websocket.Conn
	send      chan service.ProgressEvent
	closeOnce sync.Once
}

func ServeProgress(hub *Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		if hub == nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "progress hub unavailable"})
			return
		}

		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}

		client := &Client{
			hub:  hub,
			conn: conn,
			send: make(chan service.ProgressEvent, 16),
		}
		hub.Register(client)

		go client.writePump()
		go client.readPump()
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

	c.conn.SetReadLimit(512)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (c *Client) writePump() {
	if c == nil || c.conn == nil {
		return
	}
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case event, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, nil)
				return
			}
			if err := c.conn.WriteJSON(event); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
