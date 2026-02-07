package web

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"github.com/coder/websocket"
	"github.com/creack/pty/v2"
)

type resizeMsg struct {
	Type string `json:"type"`
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		log.Printf("websocket accept: %v", err)
		return
	}
	defer conn.CloseNow()

	cols := parseUint16(r.URL.Query().Get("cols"), 80)
	rows := parseUint16(r.URL.Query().Get("rows"), 24)

	exe, err := os.Executable()
	if err != nil {
		log.Printf("os.Executable: %v", err)
		conn.Close(websocket.StatusInternalError, "cannot find executable")
		return
	}

	cmd := exec.Command(exe, "tui", "--server", s.apiAddr, "--db", s.dbPath)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color", "COLORTERM=truecolor")

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: rows, Cols: cols})
	if err != nil {
		log.Printf("pty start: %v", err)
		conn.Close(websocket.StatusInternalError, "failed to start pty")
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var once sync.Once
	cleanup := func() {
		cancel()
		ptmx.Close()
		if cmd.Process != nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
	}

	// PTY -> WebSocket (binary frames to avoid UTF-8 validation issues)
	go func() {
		buf := make([]byte, 32*1024)
		for {
			n, err := ptmx.Read(buf)
			if err != nil {
				log.Printf("pty read: %v", err)
				once.Do(cleanup)
				conn.Close(websocket.StatusNormalClosure, "process exited")
				return
			}
			if err := conn.Write(ctx, websocket.MessageBinary, buf[:n]); err != nil {
				log.Printf("ws write: %v", err)
				once.Do(cleanup)
				return
			}
		}
	}()

	// WebSocket -> PTY
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			log.Printf("ws read: %v", err)
			once.Do(cleanup)
			return
		}

		msg := string(data)
		if strings.HasPrefix(msg, "{") {
			var resize resizeMsg
			if json.Unmarshal(data, &resize) == nil && resize.Type == "resize" {
				pty.Setsize(ptmx, &pty.Winsize{Rows: resize.Rows, Cols: resize.Cols})
				continue
			}
		}

		if _, err := ptmx.Write(data); err != nil {
			once.Do(cleanup)
			return
		}
	}
}

func parseUint16(s string, def uint16) uint16 {
	if s == "" {
		return def
	}
	v, err := strconv.ParseUint(s, 10, 16)
	if err != nil {
		return def
	}
	return uint16(v)
}
