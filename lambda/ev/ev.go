package main

import (
	"context"
	"encoding/xml"
	"fmt"
	"log"
	"net/http"
	"time"

	"net/url"

	"github.com/afex/hystrix-go/hystrix"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/kelseyhightower/envconfig"
	"github.com/robertacosta/go-lambda-twilio-sendgrid/adaptor/sendgrid"
	"github.com/robertacosta/go-lambda-twilio-sendgrid/model"
)

type Cfg struct {
	// API Keys
	SendGridAPIKey string `envconfig:"sendgrid_api_key" default:"api-key"`
}

func main() {
	var cfg Cfg
	err := envconfig.Process("ev", &cfg)
	if err != nil {
		log.Fatalf("error getting config: %s\n", err)
	}

	hystrixScoringConfig := hystrix.CommandConfig{
		Timeout:               2000, // Timeout request after 2 sec
		MaxConcurrentRequests: 100,  // Bulk head, max requests that can be concurrently running, all others rejected
		SleepWindow:           5000, // If circuit opens, try to close every 5 sec
		ErrorPercentThreshold: 50,   // If over 50% of the requests return an error, open circuit
	}
	emailValidator := sendgrid.NewEmailValidation(cfg.SendGridAPIKey, hystrixScoringConfig)
	emailValidatorTimeDuration := time.Duration(2000) * time.Millisecond
	emailValidatorHttpClient := &http.Client{
		Timeout: emailValidatorTimeDuration,
		Transport: &http.Transport{
			TLSHandshakeTimeout: emailValidatorTimeDuration,
		},
	}
	emailValidator.SetHttpClient(emailValidatorHttpClient)

	ev := EV{
		emailValidator: emailValidator,
	}

	lambda.Start(ev.Handler)
}

type EV struct {
	cfg Cfg

	emailValidator sendgrid.Validator
}

func (e *EV) Handler(ctx context.Context, request model.TwilioRequest) (string, error) {
	fmt.Printf("Twilio request: %+v\n", request)

	email, err := url.QueryUnescape(request.Body)
	if len(email) < 1 || err != nil {
		return wrapWithTwilioReponse("Please provide an email"), fmt.Errorf("email address was not provided")
	}

	evResponse, err := e.emailValidator.Validate(email)
	if err != nil {
		return wrapWithTwilioReponse("Could not validate email"), fmt.Errorf("email validation failed")
	}

	return wrapWithTwilioReponse(fmt.Sprintf("Score: %f", evResponse.Result.Score)), nil

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
