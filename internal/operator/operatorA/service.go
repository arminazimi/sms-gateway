package operatorA

import (
	"context"

	"sms-gateway/app"
	"sms-gateway/internal/model"
)

type OA struct{}

func (o OA) Send(ctx context.Context, s model.SMS) error {

	//return errors.New("fall down")  for test circuit breaker

	for _, v := range s.Recipients {
		app.Logger.Info("your sms has sent ",
			"user id : ", s.CustomerID,
			"msg : ", s.Text,
			"number : ", v,
			"operator:", "A")
	}

	return nil
}
