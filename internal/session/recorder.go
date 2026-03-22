package session

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/mison/antigravity-go/internal/llm"
)

type Metadata struct {
	ID            string    `json:"id"`
	CreatedAt     time.Time `json:"created_at"`
	WorkspaceRoot string    `json:"workspace_root"`
	Interface     string    `json:"interface"` // tui/web/run
	Approvals     string    `json:"approvals"`
	Provider      string    `json:"provider,omitempty"`
	Model         string    `json:"model,omitempty"`
}

type Recorder struct {
	Meta Metadata
	Dir  string

	eventsFile *os.File
	mu         sync.Mutex
}

type Event struct {
	Time time.Time `json:"time"`
	Type string    `json:"type"`
	Data any       `json:"data,omitempty"`
}

func New(rootDir string, meta Metadata) (*Recorder, error) {
	if rootDir == "" {
		return nil, fmt.Errorf("rootDir is empty")
	}
	if meta.ID == "" {
		meta.ID = generateID()
	}
	if meta.CreatedAt.IsZero() {
		meta.CreatedAt = time.Now()
	}

	dir := filepath.Join(rootDir, meta.ID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	metaPath := filepath.Join(dir, "meta.json")
	if err := writeJSON(metaPath, meta); err != nil {
		return nil, err
	}

	eventsPath := filepath.Join(dir, "events.jsonl")
	f, err := os.OpenFile(eventsPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	return &Recorder{
		Meta:       meta,
		Dir:        dir,
		eventsFile: f,
	}, nil
}

func Load(rootDir string, id string) (*Recorder, error) {
	if rootDir == "" || id == "" {
		return nil, fmt.Errorf("invalid arguments")
	}
	dir := filepath.Join(rootDir, id)
	metaPath := filepath.Join(dir, "meta.json")

	var meta Metadata
	if err := readJSON(metaPath, &meta); err != nil {
		return nil, err
	}

	eventsPath := filepath.Join(dir, "events.jsonl")
	f, err := os.OpenFile(eventsPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	return &Recorder{
		Meta:       meta,
		Dir:        dir,
		eventsFile: f,
	}, nil
}

func (r *Recorder) Append(eventType string, data any) error {
	if r == nil || r.eventsFile == nil {
		return nil
	}
	if eventType == "" {
		eventType = "unknown"
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	ev := Event{
		Time: time.Now(),
		Type: eventType,
		Data: RedactAny(data),
	}
	b, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	_, err = r.eventsFile.Write(append(b, '\n'))
	return err
}

func (r *Recorder) SaveMessages(msgs []llm.Message) error {
	if r == nil {
		return nil
	}
	redacted := make([]llm.Message, 0, len(msgs))
	for _, m := range msgs {
		cp := m
		cp.Content = RedactString(cp.Content)
		if len(cp.ToolCalls) > 0 {
			calls := make([]llm.ToolCall, len(cp.ToolCalls))
			for i, tc := range cp.ToolCalls {
				calls[i] = tc
				calls[i].Args = RedactString(calls[i].Args)
			}
			cp.ToolCalls = calls
		}
		redacted = append(redacted, cp)
	}
	path := filepath.Join(r.Dir, "messages.json")
	return writeJSON(path, redacted)
}

func (r *Recorder) LoadMessages() ([]llm.Message, error) {
	if r == nil {
		return nil, fmt.Errorf("recorder is nil")
	}
	path := filepath.Join(r.Dir, "messages.json")
	var msgs []llm.Message
	if err := readJSON(path, &msgs); err != nil {
		return nil, err
	}
	return msgs, nil
}

func (r *Recorder) Close() error {
	if r == nil || r.eventsFile == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.eventsFile.Close()
}

func List(rootDir string) ([]Metadata, error) {
	entries, err := os.ReadDir(rootDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Metadata{}, nil
		}
		return nil, err
	}

	var metas []Metadata
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		metaPath := filepath.Join(rootDir, e.Name(), "meta.json")
		var meta Metadata
		if err := readJSON(metaPath, &meta); err != nil {
			continue
		}
		metas = append(metas, meta)
	}

	sort.Slice(metas, func(i, j int) bool {
		return metas[i].CreatedAt.After(metas[j].CreatedAt)
	})
	return metas, nil
}

func generateID() string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return time.Now().Format("20060102-150405") + "-" + hex.EncodeToString(b[:])
}

func writeJSON(path string, v any) error {
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	bw := bufio.NewWriter(f)
	enc := json.NewEncoder(bw)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := bw.Flush(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

func readJSON(path string, out any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}
