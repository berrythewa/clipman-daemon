package clipboard

import (
	"container/ring"
	"sync"
	"time"
	"github.com/berrythewa/clipman-daemon/internal/types"
)

type HistoryItem struct {
	Content *types.ClipboardContent
	Time    time.Time
}

type ClipboardHistory struct {
	history *ring.Ring
	mu      sync.Mutex
	size    int
}

func NewClipboardHistory(size int) *ClipboardHistory {
	return &ClipboardHistory{
		history: ring.New(size),
		size:    size,
	}
}

func (ch *ClipboardHistory) Add(content *types.ClipboardContent) {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	ch.history.Value = &HistoryItem{
		Content: content,
		Time:    time.Now(),
	}
	ch.history = ch.history.Next()
}

func (ch *ClipboardHistory) GetLast(n int) []*HistoryItem {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	if n > ch.size {
		n = ch.size
	}

	items := make([]*HistoryItem, 0, n)
	ch.history.Do(func(v interface{}) {
		if v != nil && len(items) < n {
			items = append([]*HistoryItem{v.(*HistoryItem)}, items...)
		}
	})

	return items
}