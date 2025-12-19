package operator

import (
	"context"
	"sms-gateway/internal/model"
)

type Operator interface {
	Send(ctx context.Context, s model.SMS)
}
