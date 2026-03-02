package twilio

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"

	twilio "github.com/twilio/twilio-go"
	api "github.com/twilio/twilio-go/rest/api/v2010"
)

var (
	clientOnce sync.Once
	client     *twilio.RestClient
)

func getClient() *twilio.RestClient {
	clientOnce.Do(func() {
		client = twilio.NewRestClientWithParams(twilio.ClientParams{
			Username: twilioAccountSID(),
			Password: twilioAuthToken(),
		})
	})
	return client
}

func SMSMe(msg string) error {
	params := &api.CreateMessageParams{}
	params.SetTo(twilioPhoneNumberMe())
	params.SetFrom(twilioPhoneNumberFrom())
	params.SetBody(msg)

	resp, err := getClient().Api.CreateMessage(params)
	if err != nil {
		return fmt.Errorf("error sending sms message: %w", err)
	}

	response, _ := json.Marshal(*resp)
	log.Println("Response: " + string(response))
	return nil
}
