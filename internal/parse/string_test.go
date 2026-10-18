package parse

import (
	"testing"
)

func TestMRSBehaviorString_More(t *testing.T) {
	if MRSBehaviorDomain.String() != "domain" {
		t.Errorf("got %q", MRSBehaviorDomain.String())
	}
	if MRSBehaviorIPCIDR.String() != "ipcidr" {
		t.Errorf("got %q", MRSBehaviorIPCIDR.String())
	}
	if MRSBehavior(7).String() != "behavior(7)" {
		t.Errorf("got %q", MRSBehavior(7).String())
	}
}

func TestFormatString_More(t *testing.T) {
	if FormatLinks.String() != "links" {
		t.Errorf("got %q", FormatLinks.String())
	}
	if FormatClashYAML.String() != "clash-yaml" {
		t.Errorf("got %q", FormatClashYAML.String())
	}
	if Format(99).String() != "unknown" {
		t.Errorf("got %q", Format(99).String())
	}
}
