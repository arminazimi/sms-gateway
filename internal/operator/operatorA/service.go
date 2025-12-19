package operatorA

import (
	"context"
	"sms-gateway/internal/model"
)

type OA struct{}

func (o OA) Send(ctx context.Context, s model.SMS) {}
