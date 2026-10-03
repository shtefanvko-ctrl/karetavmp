package main

import "testing"

func TestCloudflaredLock(t *testing.T) {
	lock, err := loadCloudflaredLock()
	if err != nil {
		t.Fatalf("loadCloudflaredLock: %v", err)
	}
	if lock.Source.Release != "2026.8.2" {
		t.Fatalf("unexpected release %q", lock.Source.Release)
	}

	amd64, err := selectCloudflaredAsset(lock, "x86_64")
	if err != nil {
		t.Fatalf("select amd64: %v", err)
	}
	if amd64.SHA256 != "fcfb02b575a52ca1af2e3267af4e1517bcdeb30ac48c834c69abaed3c0576ad2" {
		t.Fatalf("unexpected amd64 sha256 %q", amd64.SHA256)
	}

	arm64, err := selectCloudflaredAsset(lock, "aarch64")
	if err != nil {
		t.Fatalf("select arm64: %v", err)
	}
	if arm64.SHA256 != "7747d94570fb390cf47dcb4f9555c193c6355cda9793f0d878d9049e5d6a7790" {
		t.Fatalf("unexpected arm64 sha256 %q", arm64.SHA256)
	}

	if _, err := selectCloudflaredAsset(lock, "riscv64"); err == nil {
		t.Fatal("unsupported architecture must fail closed")
	}
}
