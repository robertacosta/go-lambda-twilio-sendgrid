package main

import (
	"context"
	"encoding/xml"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/afex/hystrix-go/hystrix"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/kelseyhightower/envconfig"
	"github.com/robertacosta/go-lambda-twilio-sendgrid/adaptor/sendgrid"
	"github.com/robertacosta/go-lambda-twilio-sendgrid/model"
)

const (
	Invalid = "Invalid"
)

type Cfg struct {
	// API Keys
	SendGridAPIKey string `envconfig:"sendgrid_api_key" default:"api-key"`

	// Contacts
	ContactListID string `envconfig:"contact_list_id" default:"contact-list-id"`
}

func main() {
	var cfg Cfg
	err := envconfig.Process("ev", &cfg)
	if err != nil {
		log.Fatalf("error getting config: %s\n", err)
	}

	//
	hystrixEmailValidationConfig := hystrix.CommandConfig{
		Timeout:               5000, // Timeout request after 5 sec
		MaxConcurrentRequests: 100,  // Bulk head, max requests that can be concurrently running, all others rejected
		SleepWindow:           5000, // If circuit opens, try to close every 5 sec
		ErrorPercentThreshold: 50,   // If over 50% of the requests return an error, open circuit
	}
	emailValidator := sendgrid.NewEmailValidation(cfg.SendGridAPIKey, hystrixEmailValidationConfig)
	emailValidatorTimeDuration := time.Duration(5000) * time.Millisecond
	emailValidatorHttpClient := &http.Client{
		Timeout: emailValidatorTimeDuration,
		Transport: &http.Transport{
			TLSHandshakeTimeout: emailValidatorTimeDuration,
		},
	}
	emailValidator.SetHttpClient(emailValidatorHttpClient)

	hystrixContactConfig := hystrix.CommandConfig{
		Timeout:               5000, // Timeout request after 5 sec
		MaxConcurrentRequests: 100,  // Bulk head, max requests that can be concurrently running, all others rejected
		SleepWindow:           5000, // If circuit opens, try to close every 5 sec
		ErrorPercentThreshold: 50,   // If over 50% of the requests return an error, open circuit
	}
	contact := sendgrid.NewContact(cfg.ContactListID, cfg.SendGridAPIKey, hystrixContactConfig)
	contactTimeDuration := time.Duration(5000) * time.Millisecond
	contactHttpClient := &http.Client{
		Timeout: contactTimeDuration,
		Transport: &http.Transport{
			TLSHandshakeTimeout: contactTimeDuration,
		},
	}
	contact.SetHttpClient(contactHttpClient)

	ev := EV{
		emailValidator: emailValidator,
		contactAdder:   contact,
	}

	lambda.Start(ev.Handler)
}

type EV struct {
	cfg Cfg

	emailValidator sendgrid.Validator
	contactAdder   sendgrid.Adder
}

func (e *EV) Handler(ctx context.Context, request model.TwilioRequest) (string, error) {
	fmt.Printf("Twilio request: %+v\n", request)

	email, err := url.QueryUnescape(request.Body)
	if len(email) < 1 || err != nil {
		return wrapWithTwilioReponse("Please provide an email"), nil
	}

	evResponse, err := e.emailValidator.Validate(email)
	if err != nil {
		return wrapWithTwilioReponse("Could not validate email"), nil
	}

	// If result is invalid return that instead of adding to list
	evResult := evResponse.Result
	if evResult.Result == Invalid {
		errMessage := "Email likely invalid."
		if evResult.Suggestion != "" {
			errMessage += fmt.Sprintf(" Consider using %s@%s", evResult.Local, evResult.Suggestion)
		}
		if len(evResult.Reasons) > 0 {
			errMessage += " Reasons include: "
			for _, reason := range evResult.Reasons {
				errMessage += fmt.Sprintf("%s, ", reason)
			}
			errMessage = strings.TrimSuffix(errMessage, ", ")
		}

		return wrapWithTwilioReponse(errMessage), nil
	}

	// If the result is not invalid, add to contact list
	err = e.contactAdder.Add(email)
	if err != nil {
		return wrapWithTwilioReponse("Could not add email to contact list"), nil
	}

	responseMessage := fmt.Sprintf("Email added to contact list. It was considered %s with a score of %f", evResult.Result, evResult.Score)

	return wrapWithTwilioReponse(responseMessage), nil

}

func wrapWithTwilioReponse(message string) string {
	response := model.TwilioResponse{Message: message}

	output, err := xml.Marshal(response)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return ""
	}

	xmlString := []byte(xml.Header + string(output))
	return fmt.Sprintf("%s", xmlString)
}
