package domain

import (
	"errors"
	"sync"
)

var (
	ErrQueueFull    = errors.New("queue is full")
	ErrQueueEmpty   = errors.New("queue is empty")
	ErrInvalidIndex = errors.New("invalid index")
)

type Queue interface {
	Enqueue(track Track) error
	Dequeue() (Track, error)
	Peek() (Track, error)
	All() []Track
	Clear()
	Remove(index int) error
	Size() int
	IsEmpty() bool
}

type queue struct {
	mu      sync.RWMutex
	tracks  []Track
	maxSize int
}

func NewQueue(maxSize int) Queue {
	return &queue{
		tracks:  make([]Track, 0, maxSize),
		maxSize: maxSize,
	}
}

func (q *queue) Enqueue(track Track) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.tracks) >= q.maxSize {
		return ErrQueueFull
	}

	q.tracks = append(q.tracks, track)
	return nil
}

func (q *queue) Dequeue() (Track, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.tracks) == 0 {
		return Track{}, ErrQueueEmpty
	}

	track := q.tracks[0]
	q.tracks = q.tracks[1:]
	return track, nil
}

func (q *queue) Peek() (Track, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	if len(q.tracks) == 0 {
		return Track{}, ErrQueueEmpty
	}

	return q.tracks[0], nil
}

func (q *queue) All() []Track {
	q.mu.RLock()
	defer q.mu.RUnlock()

	result := make([]Track, len(q.tracks))
	copy(result, q.tracks)
	return result
}

func (q *queue) Clear() {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.tracks = q.tracks[:0]
}

func (q *queue) Remove(index int) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if index < 0 || index >= len(q.tracks) {
		return ErrInvalidIndex
	}

	q.tracks = append(q.tracks[:index], q.tracks[index+1:]...)
	return nil
}

func (q *queue) Size() int {
	q.mu.RLock()
	defer q.mu.RUnlock()

	return len(q.tracks)
}

func (q *queue) IsEmpty() bool {
	q.mu.RLock()
	defer q.mu.RUnlock()

	return len(q.tracks) == 0
}
