package operatorB

import (
	"context"
	"sms-gateway/internal/model"
)

type OB struct{}

func (o OB) Send(ctx context.Context, s model.SMS) {}
