package core

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/pocketbase/pocketbase"
)

type Instance struct {
	App  *pocketbase.PocketBase
	Port int
}

type ServerManager struct {
	instances map[string]*Instance
	mu        sync.RWMutex
}

func NewServerManager() *ServerManager {
	return &ServerManager{
		instances: make(map[string]*Instance),
	}
}

func findAvailablePort() (int, error) {
	// Let OS choose an available port by using port 0
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()

	// Get the actual address being used
	return listener.Addr().(*net.TCPAddr).Port, nil
}

func (sm *ServerManager) GetOrCreateInstance(subdomain string) (*Instance, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Check if instance exists
	if instance, exists := sm.instances[subdomain]; exists {
		return instance, nil
	}

	port, err := findAvailablePort()
	if err != nil {
		return nil, err
	}
	log.Printf("Found available port: %d", port)

	// Create new PocketBase instance
	app := pocketbase.New()

	// Start the PocketBase instance
	go func() {
		if err := app.Serve(subdomain, port); err != nil {
			log.Fatal(err)
		}
	}()

	ready := make(chan error)

	go func() {
		// Wait for server to be ready by polling health endpoint
		healthURL := fmt.Sprintf("http://localhost:%d/api/health", port)

		triesRemaining := 100
		for triesRemaining > 0 {
			log.Printf("Checking server health... (%d tries remaining)", triesRemaining)
			triesRemaining--
			resp, err := http.Get(healthURL)
			if err == nil && resp.StatusCode == 200 {
				resp.Body.Close()
				ready <- nil
				return
			}
			if resp != nil {
				resp.Body.Close()
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	if err := <-ready; err != nil {
		log.Printf("Failed to start server %s: %v", subdomain, err)
		return nil, err
	}

	instance := &Instance{
		App:  app,
		Port: port,
	}
	sm.instances[subdomain] = instance

	return instance, nil
}
