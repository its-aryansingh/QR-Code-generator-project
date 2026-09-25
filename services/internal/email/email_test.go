package email_test

import (
	"context"
	"strings"
	"testing"

	"github.com/its-aryansingh/qrit/services/internal/email"
)

func TestMemorySender(t *testing.T) {
	sender := email.NewMemorySender()
	ctx := context.Background()
	baseURL := "https://app.qrit.io"

	// 1. Verification
	if err := sender.SendVerification(ctx, "user@example.com", "verify123", baseURL); err != nil {
		t.Fatalf("SendVerification failed: %v", err)
	}

	// 2. Reset password
	if err := sender.SendPasswordReset(ctx, "user@example.com", "reset123", baseURL); err != nil {
		t.Fatalf("SendPasswordReset failed: %v", err)
	}

	// 3. Magic link
	if err := sender.SendMagicLink(ctx, "user@example.com", "magic123", baseURL); err != nil {
		t.Fatalf("SendMagicLink failed: %v", err)
	}

	// 4. Invite
	if err := sender.SendInvite(ctx, "colleague@example.com", "Aryan", "Engineering", "inv123", baseURL); err != nil {
		t.Fatalf("SendInvite failed: %v", err)
	}

	if len(sender.Emails) != 4 {
		t.Fatalf("expected 4 emails sent, got %d", len(sender.Emails))
	}

	if !strings.Contains(sender.Emails[0].Body, "https://app.qrit.io/verify-email?token=verify123") {
		t.Errorf("verification link invalid: %s", sender.Emails[0].Body)
	}
	if !strings.Contains(sender.Emails[1].Body, "https://app.qrit.io/reset-password?token=reset123") {
		t.Errorf("reset link invalid: %s", sender.Emails[1].Body)
	}
	if !strings.Contains(sender.Emails[2].Body, "https://app.qrit.io/magic?token=magic123") {
		t.Errorf("magic link invalid: %s", sender.Emails[2].Body)
	}
	if !strings.Contains(sender.Emails[3].Body, "https://app.qrit.io/invites/inv123") {
		t.Errorf("invite link invalid: %s", sender.Emails[3].Body)
	}
}
