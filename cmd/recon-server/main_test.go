package main

import "testing"

// Listening where other computers can reach the server warns (plain HTTP, SPEC-012 T-Q2).
func TestExposure(t *testing.T) {
	for addr, warns := range map[string]bool{
		"127.0.0.1:8080": false, "localhost:8080": false, "[::1]:8080": false,
		":8080": true, "0.0.0.0:8080": true, "192.168.1.10:8080": true,
	} {
		if got := exposure(addr) != ""; got != warns {
			t.Errorf("exposure(%q) warns %v, want %v", addr, got, warns)
		}
	}
}

// AC-13: the server listens on this computer only unless the operator names another address.
func TestDefaultListenIsThisComputer(t *testing.T) {
	if defaultListen != "127.0.0.1:8080" {
		t.Errorf("default listen address %q, want 127.0.0.1:8080", defaultListen)
	}
	if exposure(defaultListen) != "" {
		t.Errorf("the default address %s warns about other computers", defaultListen)
	}
}
