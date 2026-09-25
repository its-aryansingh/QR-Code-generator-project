package email

import (
	"fmt"
	"strings"
)

type Language string

const (
	LangEN Language = "en"
	LangHI Language = "hi"
)

type EmailContent struct {
	Subject string
	Text    string
	HTML    string
}

// RenderVerificationEmail renders account verification email in EN or HI.
func RenderVerificationEmail(lang Language, verifyURL string) EmailContent {
	switch lang {
	case LangHI:
		return EmailContent{
			Subject: "QRit खाता सत्यापन (Verify your QRit Account)",
			Text: fmt.Sprintf("नमस्ते,\n\nQRit में आपका स्वागत है। अपना खाता सत्यापित करने के लिए नीचे दिए गए लिंक पर क्लिक करें:\n%s\n\nयह लिंक 24 घंटे के लिए मान्य है।", verifyURL),
			HTML: fmt.Sprintf(`<!DOCTYPE html><html><body style="font-family: sans-serif; color: #111;">
<h2>QRit में आपका स्वागत है</h2>
<p>अपना खाता सत्यापित करने के लिए नीचे दिए गए बटन पर क्लिक करें:</p>
<p><a href="%s" style="background: #5b5bf7; color: #fff; padding: 10px 20px; border-radius: 8px; text-decoration: none; font-weight: bold;">खाता सत्यापित करें</a></p>
<p style="color: #666; font-size: 12px;">यह लिंक 24 घंटे में समाप्त हो जाएगा।</p>
</body></html>`, verifyURL),
		}
	default:
		return EmailContent{
			Subject: "Verify your QRit account",
			Text: fmt.Sprintf("Hello,\n\nWelcome to QRit. Please verify your email by clicking the link below:\n%s\n\nThis link expires in 24 hours.", verifyURL),
			HTML: fmt.Sprintf(`<!DOCTYPE html><html><body style="font-family: sans-serif; color: #111;">
<h2>Welcome to QRit</h2>
<p>Please click the button below to verify your email address:</p>
<p><a href="%s" style="background: #5b5bf7; color: #fff; padding: 10px 20px; border-radius: 8px; text-decoration: none; font-weight: bold;">Verify Email Address</a></p>
<p style="color: #666; font-size: 12px;">This link will expire in 24 hours.</p>
</body></html>`, verifyURL),
		}
	}
}

// RenderInviteEmail renders workspace invitation email in EN or HI.
func RenderInviteEmail(lang Language, inviterName, workspaceName, inviteURL string) EmailContent {
	switch lang {
	case LangHI:
		return EmailContent{
			Subject: fmt.Sprintf("%s ने आपको QRit पर %s टीम में आमंत्रित किया है", inviterName, workspaceName),
			Text: fmt.Sprintf("नमस्ते,\n\n%s ने आपको QRit पर '%s' कार्यक्षेत्र में शामिल होने के लिए आमंत्रित किया है।\nआमंत्रण स्वीकार करने के लिए नीचे दिए गए लिंक पर जाएं:\n%s\n\nयह आमंत्रण 7 दिनों के लिए वैध है।", inviterName, workspaceName, inviteURL),
			HTML: fmt.Sprintf(`<!DOCTYPE html><html><body style="font-family: sans-serif; color: #111;">
<h2>QRit टीम आमंत्रण</h2>
<p><strong>%s</strong> ने आपको कार्यक्षेत्र <strong>%s</strong> में शामिल होने के लिए आमंत्रित किया है।</p>
<p><a href="%s" style="background: #5b5bf7; color: #fff; padding: 10px 20px; border-radius: 8px; text-decoration: none; font-weight: bold;">आमंत्रण स्वीकार करें</a></p>
<p style="color: #666; font-size: 12px;">यह लिंक 7 दिनों के लिए वैध है।</p>
</body></html>`, inviterName, workspaceName, inviteURL),
		}
	default:
		return EmailContent{
			Subject: fmt.Sprintf("%s invited you to join %s on QRit", inviterName, workspaceName),
			Text: fmt.Sprintf("Hello,\n\n%s has invited you to join the workspace '%s' on QRit.\nTo accept the invitation, visit:\n%s\n\nThis invite is valid for 7 days.", inviterName, workspaceName, inviteURL),
			HTML: fmt.Sprintf(`<!DOCTYPE html><html><body style="font-family: sans-serif; color: #111;">
<h2>You've been invited!</h2>
<p><strong>%s</strong> has invited you to collaborate in the <strong>%s</strong> workspace on QRit.</p>
<p><a href="%s" style="background: #5b5bf7; color: #fff; padding: 10px 20px; border-radius: 8px; text-decoration: none; font-weight: bold;">Accept Invitation</a></p>
<p style="color: #666; font-size: 12px;">This invitation will expire in 7 days.</p>
</body></html>`, inviterName, workspaceName, inviteURL),
		}
	}
}

// RenderSafetyAlertEmail renders security notice when a QR destination is flagged.
func RenderSafetyAlertEmail(lang Language, qrName, shortCode, reason string) EmailContent {
	_ = strings.ToLower(string(lang))
	return EmailContent{
		Subject: fmt.Sprintf("[Security Alert] Destination flagged for QR code: %s (%s)", qrName, shortCode),
		Text: fmt.Sprintf("Security Alert:\n\nYour QR code '%s' (%s) destination was automatically paused because it was flagged by our safety scanner.\nReason: %s\n\nPlease check your workspace dashboard to update the destination URL.", qrName, shortCode, reason),
		HTML: fmt.Sprintf(`<!DOCTYPE html><html><body style="font-family: sans-serif; color: #111;">
<h2 style="color: #dc2626;">Security Alert: Dangerous Destination Flagged</h2>
<p>Your QR code <strong>%s</strong> (<code>%s</code>) destination was automatically paused because our automated scanner identified it as unsafe.</p>
<p><strong>Reason:</strong> %s</p>
<p>Please log in to your dashboard to review and update the destination URL.</p>
</body></html>`, qrName, shortCode, reason),
	}
}
