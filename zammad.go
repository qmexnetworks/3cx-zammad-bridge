package zammadbridge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/rs/zerolog/log"
)

type ZammadApiRequest struct {
	Event           string `json:"event"`
	From            string `json:"from"`
	To              string `json:"to"`
	Direction       string `json:"direction"`
	CallId          string `json:"call_id"`
	CallIdDuplicate string `json:"callid"`
	Cause           string `json:"cause,omitempty"`
	AnsweringNumber string `json:"answeringNumber,omitempty"`
	User            string `json:"user,omitempty"`
}

// ZammadNewCall notifies Zammad that a new call came in. This is the
// first call required to process calls using Zammad.
func (z *ZammadBridge) ZammadNewCall(call *CallInformation) error {
	err := z.ZammadPost(ZammadApiRequest{
		Event:           "newCall",
		From:            call.CallFrom,
		To:              call.CallTo,
		Direction:       call.Direction,
		CallId:          call.CallUID,
		AnsweringNumber: call.AgentNumber,
		User:            call.AgentName,
	})
	call.ZammadInitialized = true
	if err != nil {
		return err
	}

	return nil
}

// ZammadAnswer notifies Zammad that the existing call was now answered by
// an agent.
func (z *ZammadBridge) ZammadAnswer(call *CallInformation) error {
	var user string
	if call.Direction == "Inbound" {
		user = call.AgentName
	}

	if !call.ZammadInitialized {
		err := z.ZammadNewCall(call)
		if err != nil {
			return fmt.Errorf("unable to initialize call with Zammad: %w", err)
		}
	}

	if call.ZammadAnswered {
		return nil // Nothing to do - TODO: can we redirect the call in Zammad?
	}

	err := z.ZammadPost(ZammadApiRequest{
		Event:           "answer",
		From:            call.CallFrom,
		To:              call.CallTo,
		Direction:       call.Direction,
		CallId:          call.CallUID,
		AnsweringNumber: call.AgentNumber,
		User:            user,
	})
	call.ZammadAnswered = true

	if err != nil {
		return err
	}

	return nil
}

// ZammadHangup notifies Zammad that the call was finished with a given cause.
// Possible values for `cause` are: "cancel", "normalClearing"
func (z *ZammadBridge) ZammadHangup(call *CallInformation, cause string) error {
	if !call.ZammadInitialized {
		err := z.ZammadNewCall(call)
		if err != nil {
			return fmt.Errorf("unable to initialize call with Zammad: %w", err)
		}
	}

	return z.ZammadPost(ZammadApiRequest{
		Event:           "hangup",
		From:            call.CallFrom,
		To:              call.CallTo,
		Direction:       call.Direction,
		CallId:          call.CallUID,
		Cause:           cause,
		AnsweringNumber: call.AgentNumber,
	})
}

// ZammadPost makes a POST Request to Zammad with the given payload
func (z *ZammadBridge) ZammadPost(payload ZammadApiRequest) error {
	// Processing
	if payload.Direction == "Inbound" {
		payload.Direction = "in"
	}
	if payload.Direction == "Outbound" {
		payload.Direction = "out"
	}
	payload.CallIdDuplicate = payload.CallId

	// Actual request
	requestBody, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("unable to serialize JSON request body: %w", err)
	}

	log.Trace().Str("call_id", payload.CallId).Str("event", payload.Event).Str("from", payload.From).Str("to", payload.To).Msg("Zammad request (POST)")
	resp, err := z.ClientZammad.Post(z.Config.Zammad.Endpoint, "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		return fmt.Errorf("unable to make request: %w", err)
	}

	log.Trace().Str("call_id", payload.CallId).Str("event", payload.Event).Str("from", payload.From).Str("to", payload.To).Int("status", resp.StatusCode).Msg("Zammad response (POST)")

	if resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected response from Zammad (HTTP %d): %s", resp.StatusCode, string(data))
	}

	return nil
}
