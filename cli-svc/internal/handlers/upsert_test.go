package handlers

import "testing"

// Regression: the boot upsert once shipped every handler with enabled=false
// (the proto bool defaulted to false because the payload omitted the field),
// which disabled the whole catalog server-side. Every catalog handler must be
// advertised as enabled.
func TestUpsertPayloadEnablesAllHandlers(t *testing.T) {
	reg := NewRegistry()
	p := reg.UpsertPayload("secret")

	if p.Secret != "secret" {
		t.Fatalf("secret not propagated: %q", p.Secret)
	}
	if len(p.Handlers) == 0 {
		t.Fatal("empty handler catalog")
	}
	if len(p.Handlers) != len(reg.All()) {
		t.Fatalf("payload has %d handlers, catalog has %d", len(p.Handlers), len(reg.All()))
	}
	for _, h := range p.Handlers {
		if !h.Enabled {
			t.Errorf("handler %q advertised as disabled", h.HandlerKey)
		}
		if h.HandlerKey == "" || h.Verb == "" {
			t.Errorf("handler missing key/verb: %+v", h)
		}
	}
}
