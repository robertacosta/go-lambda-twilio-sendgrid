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
	ContactsHystrixKey = "contacts"
	ContactsEndpoint   = "https://api.sendgrid.com/v3/mc/contacts"
)

type Adder interface {
	Add(string) error
}

type Contact struct {
	listID     string
	apiKey     string
	httpClient HTTPClient
}

func NewContact(listID string, apiKey string, hystrixCfg hystrix.CommandConfig) *Contact {
	hystrix.ConfigureCommand(ContactsHystrixKey, hystrixCfg)

	return &Contact{
		listID: listID,
		apiKey: apiKey,
	}
}

func (c *Contact) SetHttpClient(httpClient HTTPClient) {
	c.httpClient = httpClient
}

func (c *Contact) Add(email string) error {
	var errResult error

	hystrix.Do(ContactsHystrixKey, func() error {
		create := model.ContactRequest{
			ListIDs: []string{c.listID},
			Contacts: []model.Contact{
				{Email: email},
			},
		}

		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(create); err != nil {
			return err
		}

		req, err := http.NewRequest("PUT", ContactsEndpoint, &buf)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		// If the response was not OK, only ding the circuit if it was 500+ status code
		if resp.StatusCode != http.StatusAccepted && (resp.StatusCode >= 500) {
			return errors.New(fmt.Sprintf("Received a non-202 status code, %d", resp.StatusCode))
		}

		return nil
	}, func(err error) error {
		// If the circuit opens, then the fallback function is called
		log.Printf("Contact Circuit Fallback, Error received: %s", err)

		errResult = err

		return nil
	})

	return errResult
}
