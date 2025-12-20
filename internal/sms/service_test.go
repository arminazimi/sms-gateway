package sms

import (
	"sms-gateway/testutil"
	"testing"

	"sms-gateway/internal/model"
)

func TestUpdateSMS_InsertAndHistory(t *testing.T) {
	ctx := testutil.EnsureSetup(t)
	s := model.SMS{CustomerID: 1, Recipients: []string{"+1", "+2"}, Type: model.NORMAL, SmsIdentifier: "id-1"}

	if err := UpdateSMS(ctx, s, Init, "opA"); err != nil {
		t.Fatalf("update sms err: %v", err)
	}

	history, err := GetUserHistory(ctx, "1", string(Init), "id-1")
	if err != nil {
		t.Fatalf("get history err: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(history))
	}
	for _, h := range history {
		if h.SmsIdentifier != "id-1" || h.Status != Init {
			t.Fatalf("unexpected history %+v", h)
		}
	}
}

func TestUpdateSMS_NoRecipients(t *testing.T) {
	ctx := testutil.EnsureSetup(t)
	s := model.SMS{CustomerID: 1, Recipients: nil, Type: model.NORMAL}
	if err := UpdateSMS(ctx, s, Init); err == nil {
		t.Fatalf("expected error for no recipients")
	}
}

func TestSendSms_Success(t *testing.T) {
	ctx := testutil.EnsureSetup(t)

	s := model.SMS{CustomerID: 1, Recipients: []string{"+1", "+2"}, Type: model.NORMAL, SmsIdentifier: "succ-1"}
	if err := sendSms(ctx, s); err != nil {
		t.Fatalf("sendSms err: %v", err)
	}

	initRows, _ := GetUserHistory(ctx, "1", string(Init), "succ-1")
	if len(initRows) != 2 {
		t.Fatalf("expected 2 init rows, got %d", len(initRows))
	}
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
