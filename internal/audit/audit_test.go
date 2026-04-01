package audit

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestFileObserver_Notify(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")
	fo := NewFileObserver(path)

	e := Event{TS: 1700000000, Action: ActionShorten, UserID: "u1", URL: "https://example.com/a"}
	if err := fo.Notify(e); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got Event
	if err := json.Unmarshal(data[:len(data)-1], &got); err != nil { // trim newline
		t.Fatal(err)
	}
	if got != e {
		t.Fatalf("got %+v want %+v", got, e)
	}
	if data[len(data)-1] != '\n' {
		t.Fatal("expected trailing newline")
	}
}

func TestHTTPObserver_Notify(t *testing.T) {
	var received Event
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type %q", ct)
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ho := NewHTTPObserver(srv.URL)
	e := Event{TS: 42, Action: ActionFollow, URL: "https://b.example/"}
	if err := ho.Notify(e); err != nil {
		t.Fatal(err)
	}
	if received != e {
		t.Fatalf("server got %+v want %+v", received, e)
	}
}

func TestPublisher_Publish_invokesObservers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.log")
	fo := NewFileObserver(path)
	pub := NewPublisher([]Observer{fo}, zap.NewNop())
	pub.Publish(Event{TS: 1, Action: ActionShorten, URL: "https://x/"})

	deadline := time.Now().Add(2 * time.Second)
	var size int64
	for time.Now().Before(deadline) {
		st, err := os.Stat(path)
		if err == nil && st.Size() > 0 {
			size = st.Size()
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if size == 0 {
		t.Fatal("file observer did not write")
	}
}
