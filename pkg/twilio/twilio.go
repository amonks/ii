package twilio

import (
	"encoding/json"
	"fmt"
	"log"

	twilio "github.com/twilio/twilio-go"
	api "github.com/twilio/twilio-go/rest/api/v2010"
	"monks.co/credentials"
)

var client *twilio.RestClient

func init() {
	client = twilio.NewRestClientWithParams(twilio.ClientParams{
		Username: credentials.TwilioAccountSID,
		Password: credentials.TwilioAuthToken,
	})
}

func SMSMe(msg string) error {
	params := &api.CreateMessageParams{}
	params.SetTo(credentials.TwilioPhoneNumberMe)
	params.SetFrom(credentials.TwilioPhoneNumberFrom)
	params.SetBody(msg)

	resp, err := client.Api.CreateMessage(params)
	if err != nil {
		return fmt.Errorf("error sending sms message: %w", err)
	}

	response, _ := json.Marshal(*resp)
	log.Println("Response: " + string(response))
	return nil
}
