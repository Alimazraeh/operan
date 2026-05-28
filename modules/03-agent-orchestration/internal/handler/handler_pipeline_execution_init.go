package handler

import (
	"github.com/operan/modules/03-agent-orchestration/internal/events"
	"github.com/operan/modules/03-agent-orchestration/internal/repository"
)

// NewPipelineHandler returns a PipelineHandler with the given stores.
func NewPipelineHandler(pl repository.PipelineStoreIface, ex repository.ExecutionStoreIface, ht repository.HumanTaskStoreIface) *PipelineHandler {
	return &PipelineHandler{
		PipelineStore:  pl,
		ExecutionStore: ex,
		HumanTaskStore: ht,
	}
}

// WithEvents sets the event publisher on the PipelineHandler.
func (h *PipelineHandler) WithEvents(e *events.Publisher) *PipelineHandler {
	h.Events = e
	return h
}

// NewExecutionHandler returns an ExecutionHandler with the given stores.
func NewExecutionHandler(ex repository.ExecutionStoreIface, pl repository.PipelineStoreIface) *ExecutionHandler {
	return &ExecutionHandler{
		ExecutionStore: ex,
		PipelineStore:  pl,
	}
}

// WithEvents sets the event publisher on the ExecutionHandler.
func (h *ExecutionHandler) WithEvents(e *events.Publisher) *ExecutionHandler {
	h.Events = e
	return h
}

// NewHumanTaskHandler returns a HumanTaskHandler with the given stores.
func NewHumanTaskHandler(ht repository.HumanTaskStoreIface, ex repository.ExecutionStoreIface) *HumanTaskHandler {
	return &HumanTaskHandler{
		HumanTaskStore: ht,
		ExecutionStore: ex,
	}
}

// WithEvents sets the event publisher on the HumanTaskHandler.
func (h *HumanTaskHandler) WithEvents(e *events.Publisher) *HumanTaskHandler {
	h.Events = e
	return h
}
