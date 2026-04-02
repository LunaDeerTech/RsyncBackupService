package ws

import (
	"testing"
	"time"

	"github.com/LunaDeerTech/RsyncBackupService/internal/config"
	executorpkg "github.com/LunaDeerTech/RsyncBackupService/internal/executor"
	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"github.com/LunaDeerTech/RsyncBackupService/internal/service"
)

func TestHubRegisterAndUnregisterClient(t *testing.T) {
	hub := NewHub()
	client := &Client{send: make(chan service.ProgressEvent, 1)}

	hub.Register(client)
	if len(hub.clients) != 1 {
		t.Fatalf("expected 1 registered client, got %d", len(hub.clients))
	}
	if _, ok := hub.clients[client]; !ok {
		t.Fatal("expected client to be tracked by hub")
	}

	hub.Unregister(client)
	if len(hub.clients) != 0 {
		t.Fatalf("expected 0 registered clients, got %d", len(hub.clients))
	}
	if _, ok := hub.clients[client]; ok {
		t.Fatal("expected client to be removed from hub")
	}
}

func TestProgressHubBroadcastsToClients(t *testing.T) {
	hub := NewHub()
	clientA := &Client{send: make(chan service.ProgressEvent, 1)}
	clientB := &Client{send: make(chan service.ProgressEvent, 1)}
	hub.Register(clientA)
	hub.Register(clientB)

	event := service.ProgressEvent{
		TaskID:        "task-1",
		InstanceID:    42,
		Percentage:    37.5,
		SpeedText:     "12 MB/s",
		RemainingText: "2m",
		Status:        model.BackupStatusRunning,
	}

	hub.Broadcast(event)

	assertHubEvent := func(name string, messages <-chan service.ProgressEvent) {
		t.Helper()

		select {
		case received := <-messages:
			if received != event {
				t.Fatalf("expected %s to receive %+v, got %+v", name, event, received)
			}
		case <-time.After(200 * time.Millisecond):
			t.Fatalf("expected %s to receive broadcast event", name)
		}
	}

	assertHubEvent("clientA", clientA.send)
	assertHubEvent("clientB", clientB.send)
}

func TestBridgeProgressBroadcastsExecutorEventsToHub(t *testing.T) {
	hub := NewHub()
	client := &Client{send: make(chan service.ProgressEvent, 1)}
	hub.Register(client)

	executorService := service.NewExecutorService(nil, config.Config{}, nil, executorpkg.NewTaskManager())
	stop := BridgeProgress(executorService, hub)
	defer stop()

	event := service.ProgressEvent{
		TaskID:        "task-7",
		InstanceID:    7,
		Percentage:    12.5,
		SpeedText:     "4 MB/s",
		RemainingText: "30s",
		Status:        model.BackupStatusRunning,
	}

	executorService.PublishProgress(event)

	select {
	case received := <-client.send:
		if received != event {
			t.Fatalf("expected bridged event %+v, got %+v", event, received)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected bridged progress event to reach hub client")
	}

	stop()
	executorService.PublishProgress(service.ProgressEvent{TaskID: "task-8", InstanceID: 8, Status: model.BackupStatusRunning})

	select {
	case received := <-client.send:
		t.Fatalf("expected no messages after bridge unsubscribe, got %+v", received)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestBridgeProgressThrottlesRunningEventsPerTask(t *testing.T) {
	originalBridgeNow := bridgeNow
	defer func() {
		bridgeNow = originalBridgeNow
	}()

	currentTime := time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC)
	bridgeNow = func() time.Time {
		return currentTime
	}

	hub := NewHub()
	client := &Client{send: make(chan service.ProgressEvent, 4)}
	hub.Register(client)

	executorService := service.NewExecutorService(nil, config.Config{}, nil, executorpkg.NewTaskManager())
	stop := BridgeProgress(executorService, hub)
	defer stop()

	first := service.ProgressEvent{TaskID: "task-9", InstanceID: 9, Percentage: 10, Status: model.BackupStatusRunning}
	second := service.ProgressEvent{TaskID: "task-9", InstanceID: 9, Percentage: 20, Status: model.BackupStatusRunning}
	third := service.ProgressEvent{TaskID: "task-9", InstanceID: 9, Percentage: 30, Status: model.BackupStatusRunning}
	terminal := service.ProgressEvent{TaskID: "task-9", InstanceID: 9, Percentage: 100, Status: model.BackupStatusSuccess}

	executorService.PublishProgress(first)
	executorService.PublishProgress(second)

	select {
	case received := <-client.send:
		if received != first {
			t.Fatalf("expected first event %+v, got %+v", first, received)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected first progress event to be broadcast")
	}

	select {
	case received := <-client.send:
		t.Fatalf("expected second running event to be throttled, got %+v", received)
	case <-time.After(50 * time.Millisecond):
	}

	currentTime = currentTime.Add(1100 * time.Millisecond)
	executorService.PublishProgress(third)

	select {
	case received := <-client.send:
		if received != third {
			t.Fatalf("expected third event %+v, got %+v", third, received)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected throttled window to reopen after one second")
	}

	executorService.PublishProgress(terminal)

	select {
	case received := <-client.send:
		if received != terminal {
			t.Fatalf("expected terminal event %+v, got %+v", terminal, received)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected terminal progress event to bypass throttling")
	}
}
