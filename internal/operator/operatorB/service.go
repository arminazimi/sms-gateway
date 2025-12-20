package operatorB

import (
	"context"
	"sms-gateway/app"
	"sms-gateway/internal/model"
)

type OB struct{}

func (o OB) Send(ctx context.Context, s model.SMS) error {

	//for test refund
	//return errors.New("fall down")

	for _, v := range s.Recipients {
		app.Logger.Info("your sms has sent ",
			"user id : ", s.CustomerID,
			"msg : ", s.Text,
			"number : ", v,
			"operator:", "B")
	}

	return nil
}
