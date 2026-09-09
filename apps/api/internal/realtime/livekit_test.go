package realtime

import (
	"strings"
	"testing"
	"time"
)

func cfg() Config {
	return Config{URL: "wss://lk.example", APIKey: "APIkey123", APISecret: "secret-secret-secret"}
}

func TestMintAndVerify(t *testing.T) {
	c := cfg()
	tok, err := c.MintRoomToken("alexandria:russian-literature:ch1", "user-1", "Margery", true, time.Hour)
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	if strings.Count(tok, ".") != 2 {
		t.Fatalf("token is not a three-part JWT: %q", tok)
	}
	cl, err := c.VerifyRoomToken(tok)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if cl.Video.Room != "alexandria:russian-literature:ch1" || !cl.Video.RoomJoin || cl.Video.CanRecord {
		t.Fatalf("grant wrong: %+v", cl.Video)
	}
	if cl.Subject != "user-1" || cl.Issuer != "APIkey123" {
		t.Fatalf("claims wrong: %+v", cl)
	}
}

func TestVerifyRejectsTamperingAndOtherSecrets(t *testing.T) {
	c := cfg()
	tok, _ := c.MintRoomToken("room", "user-1", "M", true, time.Hour)

	parts := strings.Split(tok, ".")
	tampered := parts[0] + "." + parts[1] + "x." + parts[2]
	if _, err := c.VerifyRoomToken(tampered); err == nil {
		t.Error("tampered payload verified")
	}
	other := Config{URL: "wss://x", APIKey: "APIkey123", APISecret: "wrong-secret"}
	if _, err := other.VerifyRoomToken(tok); err == nil {
		t.Error("token verified under a different secret")
	}
	if _, err := c.VerifyRoomToken("not.a.jwt.at.all"); err == nil {
		t.Error("malformed token verified")
	}
}

func TestExpiry(t *testing.T) {
	c := cfg()
	tok, err := c.MintRoomToken("room", "u", "n", true, -time.Hour)
	if err != nil {
		t.Fatalf("mint clamps negative ttl: %v", err)
	}
	if _, err := c.VerifyRoomToken(tok); err != nil {
		t.Fatalf("clamped ttl should verify, got %v", err)
	}
}

func TestDisabledMintsNothing(t *testing.T) {
	c := Config{}
	if _, err := c.MintRoomToken("room", "u", "n", true, time.Hour); err != ErrDisabled {
		t.Fatalf("disabled config minted a token: %v", err)
	}
}

func TestRoomNameIsStable(t *testing.T) {
	if RoomName("club", "ch") != RoomName("club", "ch") {
		t.Error("room names must be deterministic")
	}
	if RoomName("club", "a") == RoomName("club", "b") {
		t.Error("distinct channels must map to distinct rooms")
	}
}
