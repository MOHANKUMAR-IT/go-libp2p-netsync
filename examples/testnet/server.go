package main

import (
	"context"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/libp2p/go-netroute"
	"log"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

type Server struct {
	countmap    map[string]*atomic.Int64
	connections []*websocket.Conn
	node        *p2p
	mu          sync.Mutex
	ctx         context.Context
	cancel      context.CancelFunc
}

func (s *Server) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	bootstrapNode, err := spawnHost(ctx, true)
	if err != nil {
		log.Fatal("Error starting bootstrap node:", err)
	}
	s.ctx = ctx
	s.cancel = cancel
	s.node = bootstrapNode
	s.countmap = make(map[string]*atomic.Int64)

	http.HandleFunc("/hit", s.Handle)
	http.HandleFunc("/web", s.webHandle)
	http.HandleFunc("/reset", s.resetCount)
	http.HandleFunc("/close", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
		s.Stop()
	})
	http.HandleFunc("/publish", func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("key")
		s.publishKey(key)
		w.WriteHeader(http.StatusOK)
	})
	s.connections = make([]*websocket.Conn, 0)
	http.HandleFunc("/ws", s.wsHandle)

	r, err := netroute.New()
	if err != nil {
		log.Fatal("Error creating netroute:", err)
	}
	_, _, psrc, err := r.Route(net.IPv4zero)
	if err != nil {
		log.Fatal("Error getting default route:", err)
	}

	http.HandleFunc("/bootstrapaddr", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("/ip4/" + psrc.String() + "/tcp/6061/p2p/" + bootstrapNode.host.ID().String()))
	})

	go s.broadcastUpdates()

	go func() {
		if err := http.ListenAndServe("0.0.0.0:6060", nil); err != nil {
			log.Fatal("HTTP server error:", err)
		}
	}()

	fmt.Println("Bootstrap node started with ID:", bootstrapNode.host.ID().String(), "and address:", bootstrapNode.host.Addrs())

}

func (s *Server) Stop() {
	fmt.Println("Stopping server")
	if err := s.node.host.Close(); err != nil {
		log.Println("Error closing host:", err)
	}
	s.cancel()
}

func (s *Server) Handle(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "Missing 'key' parameter", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	if _, ok := s.countmap[key]; !ok {
		s.countmap[key] = &atomic.Int64{}
	}
	s.mu.Unlock()

	s.countmap[key].Add(1)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) webHandle(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>Hit Counter</title>
	<style>
		body {
			font-family: Arial, sans-serif;
			background-color: #f4f4f9;
			color: #333;
			display: flex;
			flex-direction: column;
			align-items: center;
			justify-content: center;
			height: 100vh;
			margin: 0;
		}
		h1 {
			color: #4CAF50;
		}
		#counter {
			font-size: 2rem;
			margin-top: 20px;
			padding: 10px 20px;
			background: #fff;
			border: 2px solid #4CAF50;
			border-radius: 8px;
			box-shadow: 0 4px 8px rgba(0, 0, 0, 0.1);
		}
		#form {
			margin-top: 20px;
		}
		input[type="text"] {
			padding: 10px;
			border: 1px solid #ddd;
			border-radius: 4px;
			width: 200px;
		}
		button {
			padding: 10px 20px;
			border: none;
			background: #4CAF50;
			color: white;
			border-radius: 4px;
			cursor: pointer;
			margin-left: 10px;
		}
		button:hover {
			background: #45a049;
		}
		#resetButton {
			background-color: #f44336; /* Red background for reset */
		}
		#resetButton:hover {
			background-color: #e53935;
		}
		.error {
			color: red;
			font-size: 1rem;
			margin-top: 20px;
		}
	</style>
</head>
<body>
	<h1>Hit Counter</h1>
	<div id="counter">Connecting...</div>
	<div id="error" class="error" style="display: none;"></div>
	<div id="form">
		<input type="text" id="keyInput" placeholder="Enter a key..." />
		<button onclick="submitKey()">Submit</button>
		<button id="resetButton" onclick="resetCounter()">Reset</button>
		<button id="closeButton" onclick="close()">close</button>
	</div>
	<script>
		// WebSocket setup
		let ws = new WebSocket("ws://localhost:6060/ws");

		ws.onopen = function() {
			document.getElementById("counter").innerHTML = "<i>Waiting for data...</i>";
		};

		ws.onmessage = function(event) {
			// Use innerHTML to render the received HTML snippet
			document.getElementById("counter").innerHTML = event.data;
		};

		ws.onerror = function() {
			document.getElementById("error").innerText = "WebSocket connection failed. Please try again later.";
			document.getElementById("error").style.display = "block";
			document.getElementById("counter").style.display = "none";
		};

		ws.onclose = function() {
			document.getElementById("error").innerText = "WebSocket connection closed.";
			document.getElementById("error").style.display = "block";
			document.getElementById("counter").style.display = "none";
		};

		// Submit key to /publish endpoint
		function submitKey() {
			const key = document.getElementById("keyInput").value;
			if (!key) {
				alert("Please enter a key.");
				return;
			}
			fetch("/publish?key=" + encodeURIComponent(key), {
				method: "POST"
			})
			.then(response => {
				if (response.ok) {
					document.getElementById("keyInput").value = ""; // Clear input
				} else {
					alert("Failed to publish key.");
				}
			})
			.catch(error => {
				console.error("Error submitting key:", error);
				alert("An error occurred while submitting the key.");
			});
		}

		// Reset the counter by sending a POST request to /reset
		function resetCounter() {
			fetch("/reset", {
				method: "POST"
			})
			.then(response => {
				if (response.ok) {
					document.getElementById("counter").innerHTML = "0"; // Reset counter on the page
				} else {
					alert("Failed to reset counter.");
				}
			})
			.catch(error => {
				console.error("Error resetting counter:", error);
				alert("An error occurred while resetting the counter.");
			});
		}
		
		function closeCounter() {
			fetch("/close", {
				method: "POST"
			})
			.then(response => {
				if (response.ok) {
				} else {
					alert("Failed to reset counter.");
				}
			})
			.catch(error => {
				console.error("Error resetting counter:", error);
				alert("An error occurred while resetting the counter.");
			});
		}
	</script>
</body>
</html>`
	w.Header().Set("Content-Type", "text/html")
	_, _ = w.Write([]byte(html))
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (s *Server) wsHandle(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}
	s.mu.Lock()
	s.connections = append(s.connections, conn)
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		for i, c := range s.connections {
			if c == conn {
				s.connections = append(s.connections[:i], s.connections[i+1:]...)
				break
			}
		}
		s.mu.Unlock()
		conn.Close()
	}()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Println("WebSocket read error:", err)
			break
		}
		log.Printf("Received WebSocket message: %s", message)
	}
}

func (s *Server) broadcastUpdates() {
	for {
		time.Sleep(5 * time.Second)

		s.mu.Lock()

		itemsCount := len(s.countmap)
		columns := 5
		if itemsCount < 5 {
			columns = itemsCount
		}

		message := fmt.Sprintf("<div style='display: grid; grid-template-columns: repeat(%d, 1fr); gap: 20px; padding: 10px;'>", columns)
		for key, counter := range s.countmap {
			message += fmt.Sprintf("<div style='background-color: #f4f4f9; padding: 10px; border-radius: 8px; box-shadow: 0 2px 5px rgba(0, 0, 0, 0.1);'><strong>%s</strong></div><div style='background-color: #fff; padding: 10px; border-radius: 8px; box-shadow: 0 2px 5px rgba(0, 0, 0, 0.1);'>%d</div>", key, counter.Load())
		}
		message += "</div>"

		for _, conn := range s.connections {
			err := conn.WriteMessage(websocket.TextMessage, []byte(message))
			if err != nil {
				log.Println("WebSocket write error:", err)
				conn.Close()
			}
		}

		s.mu.Unlock()
	}
}

func (s *Server) resetCount(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.countmap = make(map[string]*atomic.Int64)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) publishKey(key string) {
	if key == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.node.topic.Publish(ctx, []byte(key)); err != nil {
		log.Println("Error publishing key:", err)
	}
}
