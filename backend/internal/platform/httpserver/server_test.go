package httpserver_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/sklyar-vlad/HackAeza/backend/internal/platform/httpserver"
)

func TestShutdownDrainsActiveRequest(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entered := make(chan struct{})
	release := make(chan struct{})
	releaseRequest := sync.OnceFunc(func() { close(release) })
	defer releaseRequest()
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		select {
		case <-release:
		case <-r.Context().Done():
			t.Error("graceful shutdown cancelled the active request")
		}
		w.WriteHeader(http.StatusNoContent)
	}), ReadHeaderTimeout: time.Second}
	addr, done := startServer(t, ctx, server, time.Second)
	requestDone := make(chan error, 1)
	go func() {
		client := &http.Client{Timeout: 2 * time.Second}
		response, err := client.Get("http://" + addr)
		if err == nil {
			err = response.Body.Close()
			if response.StatusCode != http.StatusNoContent {
				err = errors.New("active request did not complete successfully")
			}
		}
		requestDone <- err
	}()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("request did not enter handler")
	}
	cancel()
	select {
	case err := <-done:
		t.Fatalf("server returned before active request completed: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	releaseRequest()
	select {
	case err := <-requestDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("request did not finish")
	}
	awaitServer(t, done, nil)
}

func TestShutdownDeadlineForcesConnectionClosed(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entered := make(chan struct{})
	requestCancelled := make(chan struct{})
	server := &http.Server{Handler: http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		close(entered)
		<-r.Context().Done()
		close(requestCancelled)
	}), ReadHeaderTimeout: time.Second}
	addr, done := startServer(t, ctx, server, 30*time.Millisecond)
	requestDone := make(chan error, 1)
	go func() {
		client := &http.Client{Timeout: 2 * time.Second}
		response, err := client.Get("http://" + addr)
		if response != nil {
			_ = response.Body.Close()
		}
		requestDone <- err
	}()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("request did not enter handler")
	}
	cancel()
	awaitServer(t, done, context.DeadlineExceeded)
	select {
	case <-requestCancelled:
	case <-time.After(2 * time.Second):
		t.Fatal("forced close did not cancel request")
	}
	select {
	case err := <-requestDone:
		if err == nil {
			t.Fatal("client should observe a closed connection")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("forced connection close did not reach client")
	}
}

func TestListenFailureReturnsImmediately(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	server := &http.Server{Addr: listener.Addr().String(), ReadHeaderTimeout: time.Second}
	err = httpserver.Run(context.Background(), server, quietLogger(), time.Second)
	if err == nil {
		t.Fatal("occupied port must fail startup without waiting for a signal")
	}
}

func startServer(t *testing.T, ctx context.Context, server *http.Server, timeout time.Duration) (string, <-chan error) {
	t.Helper()
	ready := make(chan string, 1)
	server.Addr = "127.0.0.1:0"
	server.BaseContext = func(listener net.Listener) context.Context {
		ready <- listener.Addr().String()
		return context.Background()
	}
	done := make(chan error, 1)
	go func() { done <- httpserver.Run(ctx, server, quietLogger(), timeout) }()
	t.Cleanup(func() { _ = server.Close() })
	select {
	case addr := <-ready:
		return addr, done
	case err := <-done:
		t.Fatalf("server failed to start: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("server did not start")
	}
	return "", nil
}

func awaitServer(t *testing.T, done <-chan error, want error) {
	t.Helper()
	select {
	case err := <-done:
		if !errors.Is(err, want) {
			t.Fatalf("server returned %v, want %v", err, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not stop")
	}
}

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
