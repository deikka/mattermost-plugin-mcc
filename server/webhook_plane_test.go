package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

// === Real tests for HMAC signature verification ===

func TestVerifyWebhookSignature_Valid(t *testing.T) {
	p := &Plugin{}
	p.configuration = &configuration{
		PlaneWebhookSecret: "my-secret-key",
	}

	body := []byte(`{"event":"issue","action":"updated"}`)
	signature := computeTestHMAC(body, "my-secret-key")

	result := p.verifyWebhookSignature(body, signature)
	assert.True(t, result, "Valid signature should be accepted")
}

func TestVerifyWebhookSignature_Invalid(t *testing.T) {
	p := &Plugin{}
	p.configuration = &configuration{
		PlaneWebhookSecret: "my-secret-key",
	}

	body := []byte(`{"event":"issue","action":"updated"}`)
	invalidSignature := "0000000000000000000000000000000000000000000000000000000000000000"

	result := p.verifyWebhookSignature(body, invalidSignature)
	assert.False(t, result, "Invalid signature should be rejected")
}

func TestVerifyWebhookSignature_NoSecret(t *testing.T) {
	p := &Plugin{}
	p.configuration = &configuration{
		PlaneWebhookSecret: "",
	}

	body := []byte(`{"event":"issue","action":"updated"}`)

	result := p.verifyWebhookSignature(body, "any-signature")
	assert.True(t, result, "Empty secret should accept all signatures (permissive mode)")
}

// computeTestHMAC generates the expected HMAC-SHA256 hex digest for test assertions.
func computeTestHMAC(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// === Skipped test stubs for Plan 03-01 ===

func TestHandlePlaneWebhook_ValidSignature(t *testing.T) {
	t.Skip("Plan 03-01: POST with valid signature returns 200")
}

func TestHandlePlaneWebhook_InvalidSignature(t *testing.T) {
	t.Skip("Plan 03-01: POST with invalid signature returns 403")
}

func TestWebhookIssueStateChange(t *testing.T) {
	t.Skip("Plan 03-01: state change event posts notification to bound channel")
}

func TestWebhookAssigneeChange(t *testing.T) {
	t.Skip("Plan 03-01: assignee change event posts notification")
}

func TestWebhookIssueComment(t *testing.T) {
	t.Skip("Plan 03-01: comment event posts truncated comment card")
}

func TestWebhookDedup(t *testing.T) {
	t.Skip("Plan 03-01: duplicate delivery ID is skipped")
}

func TestWebhookUnboundProject(t *testing.T) {
	t.Skip("Plan 03-01: event for unbound project is silently ignored")
}

func TestWebhookSelfNotificationSuppressed(t *testing.T) {
	t.Skip("Plan 03-01: plugin-originated changes are not notified")
}
