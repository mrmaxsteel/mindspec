// Package main is a compile-negative fixture (spec 127 AC-9(vi)): a
// cross-package attempt to construct guard.DestructiveCommand from a
// bare string literal, bypassing NewDestructiveCommand. It must NOT
// compile — unexported fields block a keyed composite literal from
// outside internal/guard, which is exactly the opacity property this
// bead claims. See ../../constructor_test.go's
// TestDestructiveCommand_CrossPackageBypassesFailToCompile.
package main

import "github.com/mrmaxsteel/mindspec/internal/guard"

func main() {
	_ = guard.DestructiveCommand{command: "git branch -D bead/x", valid: true} // want-compile-error: unexported field
}
