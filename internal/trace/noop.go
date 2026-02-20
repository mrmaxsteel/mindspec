package trace

// noopTracer discards all events. Zero allocation.
type noopTracer struct{}

func (noopTracer) Emit(Event)   {}
func (noopTracer) Close() error { return nil }
