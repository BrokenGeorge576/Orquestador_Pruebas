package main

import (
	"context"
	"fmt"
	"math"
	"os/exec"
	"strings"
	"sync"
	"time"

	"orquestador_p/internal/orchestrator"
)

// containerCache mapea un puerto publicado → nombre del contenedor Docker que lo expone.
// Se cachea también "" para puertos sin contenedor o cuando docker no está disponible,
// evitando reintentar el lookup en cada llamada.
var containerCache sync.Map

const maxLogLines = 200

// localPortFromHost separa "host:puerto" y determina si el host es local.
func localPortFromHost(host string) (port string, isLocal bool) {
	idx := strings.LastIndex(host, ":")
	if idx < 0 {
		return "", false
	}
	h := host[:idx]
	port = host[idx+1:]
	switch h {
	case "localhost", "127.0.0.1", "::1", "":
		return port, true
	default:
		return port, false
	}
}

// resolveContainer busca el contenedor cuyo puerto publicado coincide con port.
// Devuelve "" si no hay coincidencia o si docker no está disponible.
func resolveContainer(port string) string {
	if v, ok := containerCache.Load(port); ok {
		return v.(string)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "docker", "ps",
		"--filter", "publish="+port,
		"--format", "{{.Names}}").Output()

	name := ""
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if line = strings.TrimSpace(line); line != "" {
				name = line
				break
			}
		}
	}
	containerCache.Store(port, name)
	return name
}

// captureModuleLogs lee los logs que el contenedor del módulo produjo desde `since`
// y los reenvía a la UI como eventos de log etiquetados con el nombre del módulo.
// Es no-op (sin error) si el host es remoto, no hay contenedor, o docker no existe.
func captureModuleLogs(m ModuleConfig, since time.Time, writer orchestrator.PanelWriter) {
	port, isLocal := localPortFromHost(m.Host)
	if !isLocal || port == "" {
		return
	}

	container := resolveContainer(port)
	if container == "" {
		return
	}

	secs := int(math.Ceil(time.Since(since).Seconds())) + 1

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// CombinedOutput: muchos loggers escriben a stderr.
	out, err := exec.CommandContext(ctx, "docker", "logs",
		"--since", fmt.Sprintf("%ds", secs),
		container).CombinedOutput()
	if err != nil {
		return
	}

	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	emitted := 0
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		if emitted >= maxLogLines {
			writer.Write(orchestrator.PanelEvent{
				Type:   orchestrator.EventLog,
				Module: m.Name,
				Data:   fmt.Sprintf("…(%d líneas más truncadas)", len(lines)-emitted),
			})
			break
		}
		writer.Write(orchestrator.PanelEvent{
			Type:   orchestrator.EventLog,
			Module: m.Name,
			Data:   line,
		})
		emitted++
	}
}
