package websocket

import (
	"encoding/json"
	"errors"
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
		if err := c.Conn.Close(); err != nil && !errors.Is(err, websocket.ErrCloseSent) {
			logrus.WithError(err).Debugf("Failed to close UI connection for client %s", c.ID)
		}
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	if err := c.Conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		logrus.WithError(err).Warnf("Failed to set initial read deadline for UI client %s", c.ID)
	}
	c.Conn.SetPongHandler(func(string) error {
		if err := c.Conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
			logrus.WithError(err).Warnf("Failed to extend read deadline for UI client %s", c.ID)
		}
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
		if err := c.Conn.Close(); err != nil && !errors.Is(err, websocket.ErrCloseSent) {
			logrus.WithError(err).Debugf("Failed to close UI connection for client %s", c.ID)
		}
	}()

	for {
		select {
		case message, ok := <-c.Send:
			if err := c.Conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				logrus.WithError(err).Warnf("Failed to set write deadline for UI client %s", c.ID)
				return
			}
			if !ok {
				if err := c.Conn.WriteMessage(websocket.CloseMessage, []byte{}); err != nil {
					logrus.WithError(err).Debugf("Failed to send close message to UI client %s", c.ID)
				}
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			if _, err := w.Write(message); err != nil {
				logrus.WithError(err).Debugf("Failed to write message to UI client %s", c.ID)
				_ = w.Close()
				return
			}

			// Add queued messages to the current websocket message
			n := len(c.Send)
		drain:
			for i := 0; i < n; i++ {
				select {
				case queuedMessage := <-c.Send:
					if _, err := w.Write([]byte{'\n'}); err != nil {
						logrus.WithError(err).Debugf("Failed to write queued separator to UI client %s", c.ID)
						break drain
					}
					if _, err := w.Write(queuedMessage); err != nil {
						logrus.WithError(err).Debugf("Failed to write queued message to UI client %s", c.ID)
						break drain
					}
				default:
					// No more messages available
					break drain
				}
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			if err := c.Conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				logrus.WithError(err).Warnf("Failed to set ping write deadline for UI client %s", c.ID)
				return
			}
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
