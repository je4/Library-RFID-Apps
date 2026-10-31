package barcodescanner

import (
	"context"
	"strings"
	"sync"
)

// BarcodeReaderRegistry manages the collection of all configured barcode reader instances.
type BarcodeReaderRegistry struct {
	mu      sync.RWMutex
	readers map[string]*BarcodeReaderInstance
	order   []string
}

// NewBarcodeReaderRegistry creates a new empty reader registry.
func NewBarcodeReaderRegistry() *BarcodeReaderRegistry {
	return &BarcodeReaderRegistry{
		readers: make(map[string]*BarcodeReaderInstance),
	}
}

// RegisterReader registers a reader instance with the registry.
func (r *BarcodeReaderRegistry) RegisterReader(inst *BarcodeReaderInstance) {
	if inst == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	id := inst.GetReaderID()
	lowerID := strings.ToLower(id)
	if _, exists := r.readers[lowerID]; !exists {
		r.order = append(r.order, lowerID)
	}
	r.readers[lowerID] = inst
}

// GetReader retrieves a reader instance by readerID (case-insensitive).
func (r *BarcodeReaderRegistry) GetReader(readerID string) (*BarcodeReaderInstance, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	inst, ok := r.readers[strings.ToLower(strings.TrimSpace(readerID))]
	return inst, ok
}

// GetAllReaders returns all registered reader instances in registration order.
func (r *BarcodeReaderRegistry) GetAllReaders() []*BarcodeReaderInstance {
	r.mu.RLock()
	defer r.mu.RUnlock()

	res := make([]*BarcodeReaderInstance, 0, len(r.order))
	for _, id := range r.order {
		if inst, ok := r.readers[id]; ok {
			res = append(res, inst)
		}
	}
	return res
}

// GetDefaultReader returns the first registered reader or nil if registry is empty.
func (r *BarcodeReaderRegistry) GetDefaultReader() *BarcodeReaderInstance {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.order) > 0 {
		return r.readers[r.order[0]]
	}
	return nil
}

// StartAll starts all enabled reader instances.
func (r *BarcodeReaderRegistry) StartAll(ctx context.Context, onScan ScanCallback) error {
	for _, inst := range r.GetAllReaders() {
		if inst.GetConfig().Enabled {
			if err := inst.Start(ctx, onScan); err != nil {
				return err
			}
		}
	}
	return nil
}

// StopAll gracefully terminates all registered reader instances.
func (r *BarcodeReaderRegistry) StopAll() {
	for _, inst := range r.GetAllReaders() {
		inst.Stop()
	}
}
