package sms

import (
	"sms-gateway/testutil"
	"testing"

	"sms-gateway/internal/model"
)

func TestUpdateSMS_InsertAndHistory(t *testing.T) {
	ctx := testutil.EnsureSetup(t)
	s := model.SMS{CustomerID: 1, Recipients: []string{"+1", "+2"}, Type: model.NORMAL, SmsIdentifier: "id-1"}

	if err := InsertPending(ctx, s); err != nil {
		t.Fatalf("insert pending err: %v", err)
	}

	history, err := GetUserHistory(ctx, "1", string(Pending), "id-1")
	if err != nil {
		t.Fatalf("get history err: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(history))
	}
	for _, h := range history {
		if h.SmsIdentifier != "id-1" || h.Status != Pending {
			t.Fatalf("unexpected history %+v", h)
		}
	}
}

func TestUpdateSMS_NoRecipients(t *testing.T) {
	ctx := testutil.EnsureSetup(t)
	s := model.SMS{CustomerID: 1, Recipients: nil, Type: model.NORMAL}
	if err := InsertPending(ctx, s); err == nil {
		t.Fatalf("expected error for no recipients")
	}
}

func TestSendSms_Success(t *testing.T) {
	ctx := testutil.EnsureSetup(t)

	s := model.SMS{CustomerID: 1, Recipients: []string{"+1", "+2"}, Type: model.NORMAL, SmsIdentifier: "succ-1"}
	if err := InsertPending(ctx, s); err != nil {
		t.Fatalf("insert pending err: %v", err)
	}
	if err := sendSms(ctx, s); err != nil {
		t.Fatalf("sendSms err: %v", err)
	}

	// After processing, final state should be DONE.
	doneRows, _ := GetUserHistory(ctx, "1", string(Done), "succ-1")
	if len(doneRows) != 2 {
		t.Fatalf("expected 2 done rows, got %d", len(doneRows))
	}
	for _, h := range doneRows {
		if h.Provider != "operatorA" {
			t.Fatalf("unexpected provider %q", h.Provider)
		}
	}
}

func TestSendSms_NoRecipients(t *testing.T) {
	ctx := testutil.EnsureSetup(t)

	s := model.SMS{CustomerID: 3, Recipients: nil, Type: model.NORMAL, SmsIdentifier: "no-recips"}
	if err := sendSms(ctx, s); err == nil {
		t.Fatalf("expected error for no recipients")
	}
}
