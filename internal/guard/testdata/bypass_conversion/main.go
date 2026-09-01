// Package main is a compile-negative fixture (spec 127 AC-9(vi)): an
// explicit conversion from a plain string to guard.DestructiveCommand.
// It must NOT compile — struct and string have different underlying
// types, so no conversion exists, unlike the rejected named-string-
// type alternative (D-r4-2's compile probe: explicit conversions ARE
// accepted for a named string type, which is exactly why that design
// was rejected in favor of this opaque struct). See
// ../../constructor_test.go's
// TestDestructiveCommand_CrossPackageBypassesFailToCompile.
package main

import "github.com/mrmaxsteel/mindspec/internal/guard"

func main() {
	_ = guard.DestructiveCommand("git branch -D bead/x")
}
