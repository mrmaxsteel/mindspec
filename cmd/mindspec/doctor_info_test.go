package main

import (
	"bytes"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/doctor"
)

func TestDoctorStatusTag_Info(t *testing.T) {
	if tag := statusTag(doctor.Info); tag != "[INFO]" {
		t.Fatalf("statusTag(Info) = %q", tag)
	}
	var out bytes.Buffer
	printDoctorChecks(&out, []doctor.Check{{Name: "check", Status: doctor.Info, Message: "verify before close"}})
	if got := out.String(); got != "check: [INFO] verify before close\n" {
		t.Fatalf("rendered Info = %q", got)
	}
}
