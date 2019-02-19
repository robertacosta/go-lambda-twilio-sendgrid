package sendgrid

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/afex/hystrix-go/hystrix"
	"github.com/robertacosta/go-lambda-twilio-sendgrid/model"
)

const (
	EmailValidationHystrixKey = "email_validation"
	EmailValidationEndpoint   = "https://api.sendgrid.com/v3/validations/email"
)

type Validator interface {
	Validate(string) (*model.EmailValidationResponse, error)
}

type EmailValidation struct {
	apiKey     string
	httpClient HTTPClient
}

func NewEmailValidation(apiKey string, hystrixCfg hystrix.CommandConfig) *EmailValidation {
	hystrix.ConfigureCommand(EmailValidationHystrixKey, hystrixCfg)

	return &EmailValidation{
		apiKey: apiKey,
	}
}

func (e *EmailValidation) SetHttpClient(httpClient HTTPClient) {
	e.httpClient = httpClient
}

func (e *EmailValidation) Validate(email string) (*model.EmailValidationResponse, error) {
	result := &model.EmailValidationResponse{}
	var errResult error

	hystrix.Do(EmailValidationHystrixKey, func() error {
		jsonStr := []byte(fmt.Sprintf(`{"email":"%s"}`, email))

		req, err := http.NewRequest("POST", EmailValidationEndpoint, bytes.NewBuffer(jsonStr))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", e.apiKey))

		resp, err := e.httpClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		// If the response was not OK, only ding the circuit if it was 500+ status code
		if resp.StatusCode != http.StatusOK && (resp.StatusCode >= 500) {
			return errors.New(fmt.Sprintf("Received a non-200 status code, %d", resp.StatusCode))
		}

		err = json.NewDecoder(resp.Body).Decode(result)
		if err != nil {
			return err
		}

		return nil
	}, func(err error) error {
		// If the circuit opens, then the fallback function is called
		log.Printf("Circuit Fallback, Error received: %s", err)

		errResult = err

		return nil
	})

	// If we got an error, set the result as nil
	if errResult != nil {
		result = nil
	}

	return result, errResult
}
