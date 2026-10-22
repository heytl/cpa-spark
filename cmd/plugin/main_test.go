package main

import (
	"errors"
	"testing"

	"cpa-spark/internal/warmup"
)

func TestHostReplyDecoding(t *testing.T) {
	var result struct{ Value int }
	if err := decodeHostReply([]byte(`{"ok":true,"result":{"Value":7}}`), false, &result); err != nil || result.Value != 7 {
		t.Fatalf("valid callback result: %+v, %v", result, err)
	}
	if err := decodeHostReply([]byte(`{"ok":true,"result":{}}`), false, nil); err != nil {
		t.Fatal(err)
	}
	for _, reply := range []struct {
		body      string
		transport bool
	}{
		{`not-json`, false},
		{`{"ok":true,"result":{}}`, true},
		{`{"ok":true,"result":"invalid result shape"}`, false},
	} {
		if err := decodeHostReply([]byte(reply.body), reply.transport, &result); err == nil {
			t.Errorf("accepted invalid callback: %+v", reply)
		}
	}
	var hostErr *warmup.HostError
	err := decodeHostReply([]byte(`{"ok":false,"error":{"code":"rate_limited","http_status":429}}`), true, nil)
	if !errors.As(err, &hostErr) || hostErr.Code != "rate_limited" || hostErr.Status != 429 {
		t.Fatalf("lost structured host error: %v", err)
	}
}
