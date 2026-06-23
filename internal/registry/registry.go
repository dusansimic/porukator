// Package registry tracks which client devices currently hold an open job
// stream and routes jobs to them. It is the in-memory source of truth for the
// "online" flag; persistent state lives in Postgres.
package registry

import (
	"sync"

	porukatorv1 "github.com/dusansimic/porukator/gen/go/porukator/v1"
)

// jobBuffer bounds how many undelivered jobs a connected client may queue
// in-memory before pushes start failing (and messages are left pending for the
// client to pick up on its next connect).
const jobBuffer = 1024

// conn is a single active stream for a client. The pointer identity lets a
// later reconnect detect and supersede an earlier one without racing.
type conn struct {
	jobs chan *porukatorv1.Job
}

// Registry is safe for concurrent use.
type Registry struct {
	mu      sync.Mutex
	clients map[string]*conn
}

func New() *Registry {
	return &Registry{clients: make(map[string]*conn)}
}

// Register marks a client online and returns its job channel plus a release
// function to call (deferred) when the stream ends. If the client already had
// an open stream, the previous one is superseded: its channel is closed so its
// handler unwinds.
func (r *Registry) Register(id string) (<-chan *porukatorv1.Job, func()) {
	c := &conn{jobs: make(chan *porukatorv1.Job, jobBuffer)}

	r.mu.Lock()
	if old, ok := r.clients[id]; ok {
		close(old.jobs)
	}
	r.clients[id] = c
	r.mu.Unlock()

	release := func() {
		r.mu.Lock()
		// Only remove if we are still the current connection; a newer
		// reconnect may have already replaced us.
		if cur, ok := r.clients[id]; ok && cur == c {
			delete(r.clients, id)
		}
		r.mu.Unlock()
	}
	return c.jobs, release
}

// Push delivers a job to a connected client. It returns false if the client is
// offline or its buffer is full, in which case the caller should leave the
// message pending.
func (r *Registry) Push(id string, job *porukatorv1.Job) bool {
	r.mu.Lock()
	c, ok := r.clients[id]
	r.mu.Unlock()
	if !ok {
		return false
	}
	select {
	case c.jobs <- job:
		return true
	default:
		return false
	}
}

// IsOnline reports whether a client currently holds a stream.
func (r *Registry) IsOnline(id string) bool {
	r.mu.Lock()
	_, ok := r.clients[id]
	r.mu.Unlock()
	return ok
}

// OnlineSet returns the set of online client ids.
func (r *Registry) OnlineSet() map[string]bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string]bool, len(r.clients))
	for id := range r.clients {
		out[id] = true
	}
	return out
}
