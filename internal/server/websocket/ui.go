package websocket

import (
	"encoding/json"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

// readPump pumps messages from the websocket connection to the hub
func (c *UIConnection) readPump() {
	// Add panic recovery
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("Panic in UI readPump for client %s: %v", c.ID, r)
		}
		c.Hub.unregisterUI <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, messageData, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logrus.Errorf("WebSocket error from UI client %s: %v", c.ID, err)
			}
			break
		}

		// Handle UI client messages (for future implementation)
		var message map[string]interface{}
		if err := json.Unmarshal(messageData, &message); err != nil {
			logrus.Errorf("Failed to parse message from UI client %s: %v", c.ID, err)
			continue
		}

		logrus.Debugf("Received message from UI client %s: %v", c.ID, message)
	}
}

// writePump pumps messages from the hub to the websocket connection
func (c *UIConnection) writePump() {
	// Add panic recovery
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("Panic in UI writePump for client %s: %v", c.ID, r)
		}
	}()

	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages to the current websocket message
			n := len(c.Send)
		drain:
			for i := 0; i < n; i++ {
				select {
				case queuedMessage := <-c.Send:
					w.Write([]byte{'\n'})
					w.Write(queuedMessage)
				default:
					// No more messages available
					break drain
				}
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// startPumps starts the read and write pumps with duplicate prevention
func (c *UIConnection) startPumps() {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Prevent duplicate pump creation
	if c.PumpsStarted {
		logrus.Warnf("Pumps already started for UI client %s, skipping duplicate start", c.ID)
		return
	}

	c.PumpsStarted = true
	logrus.Debugf("Starting pumps for UI client %s", c.ID)

	// Start goroutines for reading and writing
	go c.writePump()
	go c.readPump()
}
