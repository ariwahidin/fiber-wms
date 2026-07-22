// controllers/realtime_controller.go
package controllers

import (
	"fiber-app/controllers/realtime"

	"github.com/gofiber/contrib/websocket"
)

func RealtimeWSHandler() func(*websocket.Conn) {
	return func(c *websocket.Conn) {
		realtime.GlobalHub.Register(c)
		defer realtime.GlobalHub.Unregister(c)

		for {
			// baca pesan cuma buat detect disconnect (client gak perlu kirim apa2)
			if _, _, err := c.ReadMessage(); err != nil {
				break
			}
		}
	}
}
