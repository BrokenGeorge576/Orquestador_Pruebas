package webserver

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"orquestador_p/internal/orchestrator"
)

type Server struct {
	mux      *http.ServeMux
	fn       orchestrator.OrchestratorFn
	staticFS embed.FS

	uploadMu sync.Mutex
	uploads  map[string]string // token → temp file path
}

const contentType = "Content-Type"

func New(fn orchestrator.OrchestratorFn, staticFS embed.FS) http.Handler {
	s := &Server{
		fn:       fn,
		staticFS: staticFS,
		mux:      http.NewServeMux(),
		uploads:  make(map[string]string),
	}
	s.mux.HandleFunc("/", s.indexHandler)
	s.mux.HandleFunc("/api/upload", s.uploadHandler)
	s.mux.HandleFunc("/api/run", s.runHandler)
	return s.mux
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data, err := s.staticFS.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set(contentType, "text/html; charset=utf-8")
	w.Write(data)
}

func (s *Server) uploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseMultipartForm(64 << 20); err != nil {
		http.Error(w, "error leyendo form", http.StatusBadRequest)
		return
	}

	src, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "campo 'file' requerido", http.StatusBadRequest)
		return
	}
	defer src.Close()

	// Usamos un directorio temporal para preservar el nombre original del archivo,
	// ya que algunos servicios REST validan el nombre del fichero en el form-data.
	tmpDir, err := os.MkdirTemp("", "orq-*")
	if err != nil {
		http.Error(w, "error creando directorio temporal", http.StatusInternalServerError)
		return
	}

	tmpPath := tmpDir + "/" + header.Filename
	f, err := os.Create(tmpPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		http.Error(w, "error creando archivo temporal", http.StatusInternalServerError)
		return
	}

	if _, err := io.Copy(f, src); err != nil {
		f.Close()
		os.RemoveAll(tmpDir)
		http.Error(w, "error guardando archivo", http.StatusInternalServerError)
		return
	}
	f.Close()

	token := newToken()
	s.uploadMu.Lock()
	s.uploads[token] = tmpPath
	s.uploadMu.Unlock()

	w.Header().Set(contentType, "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": token})
}

func (s *Server) runHandler(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")

	s.uploadMu.Lock()
	filePath, ok := s.uploads[token]
	s.uploadMu.Unlock()

	if !ok {
		http.Error(w, "token inválido o expirado", http.StatusBadRequest)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming no soportado", http.StatusInternalServerError)
		return
	}

	w.Header().Set(contentType, "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	defer func() {
		s.uploadMu.Lock()
		delete(s.uploads, token)
		s.uploadMu.Unlock()
		os.RemoveAll(filepath.Dir(filePath))
	}()

	writer := &sseWriter{w: w, flusher: flusher}

	if err := s.fn(r.Context(), filePath, writer); err != nil {
		writer.Write(orchestrator.PanelEvent{
			Type: orchestrator.EventLog,
			Data: "ERROR: " + err.Error(),
		})
	}

	fmt.Fprintf(w, "event: done\ndata: {}\n\n")
	flusher.Flush()
}

func newToken() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

type sseWriter struct {
	mu      sync.Mutex
	w       http.ResponseWriter
	flusher http.Flusher
}

func (sw *sseWriter) Write(event orchestrator.PanelEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	sw.mu.Lock()
	defer sw.mu.Unlock()
	fmt.Fprintf(sw.w, "event: %s\ndata: %s\n\n", event.Type, string(data))
	sw.flusher.Flush()
}
