package stdwebhook

import (
	"net/http"
	"testing"
	"time"
)

// Test vector from the Standard Webhooks reference libraries.
func TestSpecVector(t *testing.T) {
	secret := "whsec_MfKQ9r8GKYqrTwjUPD8ILPZIo2LaLaSw"
	ts := time.Unix(1614265330, 0)
	sig, err := Sign(secret, "msg_p5jXN8AQM9LWM0D4loKWxJek", ts, []byte(`{"test": 2432232314}`))
	if err != nil {
		t.Fatal(err)
	}
	if sig != "v1,g0hM9SsE+OTPJTGt/tmIKtSyZlE3uFJELVlNIOLJ1OE=" {
		t.Fatalf("signature %s", sig)
	}
	h := http.Header{}
	_ = SetHeaders(h, secret, "msg_p5jXN8AQM9LWM0D4loKWxJek", ts, []byte(`{"test": 2432232314}`))
	if err := Verify(secret, h, []byte(`{"test": 2432232314}`), 5*time.Minute, ts.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if Verify(secret, h, []byte(`{"test": 1}`), 5*time.Minute, ts) == nil {
		t.Fatal("tampered body accepted")
	}
	if Verify(secret, h, []byte(`{"test": 2432232314}`), 5*time.Minute, ts.Add(time.Hour)) == nil {
		t.Fatal("stale timestamp accepted")
	}
	h.Set("webhook-signature", "v1,bad "+sig)
	if Verify(secret, h, []byte(`{"test": 2432232314}`), 5*time.Minute, ts) != nil {
		t.Fatal("rotation: second signature must be accepted")
	}
	if _, err := Sign("nope", "x", ts, nil); err == nil {
		t.Fatal("secret without prefix accepted")
	}
}
