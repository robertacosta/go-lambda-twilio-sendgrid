package model

import "encoding/xml"

type TwilioRequest struct {
	Body string `json:"Body"`
}

type TwilioResponse struct {
	XMLName xml.Name `xml:"Response"`
	Message string   `xml:"Message"`
}

type EmailValidationResponse struct {
	Result EmailValidationResult `json:"result"`
}

type EmailValidationResult struct {
	Email      string   `json:"email"`
	Result     string   `json:"result"`
	Score      float64  `json:"score"`
	Local      string   `json:"local,omitempty"`
	Host       string   `json:"host,omitempty"`
	Suggestion string   `json:"suggestion,omitempty"`
	Reasons    []string `json:"reasons,omitempty"`
}

type ContactRequest struct {
	ListIDs  []string  `json:"list_ids"`
	Contacts []Contact `json:"contacts"`
}

type Contact struct {
	Email string `json:"email"`
}
