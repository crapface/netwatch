package ui

import (
	"strconv"
	"sync"

	"github.com/lxn/walk"

	"netwatch/internal/i18n"
	"netwatch/internal/model"
)

// Column indices for the host table.
const (
	colStatus = iota
	colIP
	colHostname
	colLabel
	colVendor
	colMAC
	colPorts
	colNotes
	colID
	hostColCount
)

// HostModel is a walk TableModel backed by the discovered host list. It is
// shared by both the Scanner and Monitor table views.
type HostModel struct {
	walk.TableModelBase
	mu         sync.Mutex
	items      []model.Host
	index      map[string]int
	portLabels map[int]string
}

// NewHostModel returns an empty host model.
func NewHostModel() *HostModel {
	return &HostModel{index: map[string]int{}, portLabels: map[int]string{}}
}

// SetPortLabels updates the port->label map used to render the Ports column.
func (m *HostModel) SetPortLabels(labels map[int]string) {
	m.mu.Lock()
	m.portLabels = labels
	m.mu.Unlock()
	m.PublishRowsReset()
}

// SetItems replaces the entire host list and refreshes the views.
func (m *HostModel) SetItems(items []model.Host) {
	m.mu.Lock()
	m.items = make([]model.Host, len(items))
	copy(m.items, items)
	m.index = make(map[string]int, len(items))
	for i := range m.items {
		m.index[m.items[i].ID] = i
	}
	m.mu.Unlock()
	m.PublishRowsReset()
}

// Items returns a copy of the current host list.
func (m *HostModel) Items() []model.Host {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]model.Host, len(m.items))
	copy(out, m.items)
	return out
}

// RowCount implements walk.TableModel.
func (m *HostModel) RowCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.items)
}

// StatusOf returns the monitoring status for a row (used by the cell styler).
func (m *HostModel) StatusOf(row int) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if row < 0 || row >= len(m.items) {
		return ""
	}
	return m.items[row].Status
}

// UpdateStatus updates a host's runtime fields by ID and refreshes its row.
func (m *HostModel) UpdateStatus(id, status string, misses int, alerted bool) {
	m.mu.Lock()
	r, ok := m.index[id]
	if ok {
		m.items[r].Status = status
		m.items[r].ConsecutiveMisses = misses
		m.items[r].AlertedDown = alerted
	}
	m.mu.Unlock()
	if ok {
		m.PublishRowChanged(r)
	}
}

// Value implements walk.TableModel.
func (m *HostModel) Value(row, col int) interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	if row < 0 || row >= len(m.items) {
		return ""
	}
	h := m.items[row]
	switch col {
	case colStatus:
		return statusLabel(h.Status)
	case colIP:
		return h.IP
	case colHostname:
		return h.Hostname
	case colLabel:
		return h.Label
	case colVendor:
		return h.Vendor
	case colMAC:
		return h.MAC
	case colPorts:
		return model.PortsLabeled(h.OpenPorts, m.portLabels)
	case colNotes:
		return h.Notes
	case colID:
		return h.ID
	}
	return ""
}

// statusLabel renders a localized, glyph-prefixed status string.
func statusLabel(s string) string {
	switch s {
	case model.StatusUp:
		return "● " + i18n.T("status.up")
	case model.StatusDown:
		return "● " + i18n.T("status.down")
	default:
		return i18n.T("status.unknown")
	}
}

// EventModel is a walk TableModel backed by the monitoring event log.
type EventModel struct {
	walk.TableModelBase
	mu    sync.Mutex
	items []model.MonitorEvent
}

// NewEventModel returns an empty event model.
func NewEventModel() *EventModel { return &EventModel{} }

// SetItems replaces the entire event list.
func (m *EventModel) SetItems(items []model.MonitorEvent) {
	m.mu.Lock()
	m.items = make([]model.MonitorEvent, len(items))
	copy(m.items, items)
	m.mu.Unlock()
	m.PublishRowsReset()
}

// Append adds one event and refreshes the view.
func (m *EventModel) Append(e model.MonitorEvent) {
	m.mu.Lock()
	m.items = append(m.items, e)
	m.mu.Unlock()
	m.PublishRowsReset()
}

// RowCount implements walk.TableModel.
func (m *EventModel) RowCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.items)
}

// Value implements walk.TableModel.
func (m *EventModel) Value(row, col int) interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	if row < 0 || row >= len(m.items) {
		return ""
	}
	e := m.items[row]
	switch col {
	case 0:
		return e.Time.Format("2006-01-02 15:04:05")
	case 1:
		if e.Hostname != "" {
			return e.Hostname + " (" + e.IP + ")"
		}
		return e.IP
	case 2:
		return e.Type
	case 3:
		return e.Detail
	}
	return ""
}

func itoa(i int) string { return strconv.Itoa(i) }
