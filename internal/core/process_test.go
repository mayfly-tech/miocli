package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestManagerCloseStopsOwnedProcess(t *testing.T) {
	log, err := os.Create(filepath.Join(t.TempDir(), "mihomo.log"))
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(os.Args[0], "-test.run=^TestOwnedProcessHelper$")
	command.Env = append(os.Environ(), "MIOCLI_OWNED_PROCESS_HELPER=1")
	if err := command.Start(); err != nil {
		log.Close()
		t.Fatal(err)
	}
	manager := &Manager{
		command: command,
		done:    make(chan struct{}),
		log:     log,
	}
	go func() {
		manager.waitErr = command.Wait()
		close(manager.done)
	}()

	if err := manager.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-manager.done:
	case <-time.After(2 * time.Second):
		t.Fatal("owned process did not stop")
	}
	if err := manager.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
}

func TestOwnedProcessHelper(t *testing.T) {
	if os.Getenv("MIOCLI_OWNED_PROCESS_HELPER") != "1" {
		return
	}
	select {}
}
