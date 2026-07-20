package main

// panel_disposition_gatekeys_test.go pins the parity the internal/panel
// leaf-import invariant forces (TestPanelLeafImports_StdlibPlusTermsafeOnly,
// spec 116 AC7 / spec 120 R2): internal/panel may not import
// internal/config, so disposition.go's CanonicalGateKeys is a commented,
// deliberate mirror of config.PanelGateKeys rather than a direct
// reference. This test lives here — in cmd/mindspec, which already
// imports both packages (see panel.go's --gate handling) — because it is
// the one place in the module that CAN see both copies at once. If this
// test ever fails, someone changed one copy without the other.

import (
	"reflect"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/panel"
)

func TestDispositionGateKeysMirrorConfig(t *testing.T) {
	if !reflect.DeepEqual(panel.CanonicalGateKeys, config.PanelGateKeys) {
		t.Fatalf("panel.CanonicalGateKeys = %v, want byte-for-byte parity with config.PanelGateKeys = %v", panel.CanonicalGateKeys, config.PanelGateKeys)
	}
}
