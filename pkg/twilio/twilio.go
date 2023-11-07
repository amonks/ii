package twilio

import (
	"encoding/json"
	"fmt"
	"log"

	twilio "github.com/twilio/twilio-go"
	api "github.com/twilio/twilio-go/rest/api/v2010"
)

var client *twilio.RestClient

func init() {
	client = twilio.NewRestClientWithParams(twilio.ClientParams{
		Username: accountSid,
		Password: authToken,
	})
}

func SMSMe(msg string) error {
	params := &api.CreateMessageParams{}
	params.SetTo(phoneNumberMe)
	params.SetFrom(phoneNumberFrom)
	params.SetBody(msg)

	resp, err := client.Api.CreateMessage(params)
	if err != nil {
		return fmt.Errorf("error sending sms message: %w", err)
	}

	response, _ := json.Marshal(*resp)
	log.Println("Response: " + string(response))
	return nil
}
