package zammadbridge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
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
	return z.ZammadPost(ZammadApiRequest{
		Event:           "newCall",
		From:            call.CallFrom,
		To:              call.CallTo,
		Direction:       call.Direction,
		CallId:          call.CallUID,
		AnsweringNumber: call.AgentNumber,
		User:            call.AgentName,
	})
}

// ZammadAnswer notifies Zammad that the existing call was now answered by
// an agent.
func (z *ZammadBridge) ZammadAnswer(call *CallInformation) error {
	var user string
	if call.Direction == "Inbound" {
		user = call.AgentName
	}

	return z.ZammadPost(ZammadApiRequest{
		Event:           "answer",
		From:            call.CallFrom,
		To:              call.CallTo,
		Direction:       call.Direction,
		CallId:          call.CallUID,
		AnsweringNumber: call.AgentNumber,
		User:            user,
	})
}

// ZammadHangup notifies Zammad that the call was finished with a given cause.
// Possible values for `cause` are: "cancel", "normalClearing"
func (z *ZammadBridge) ZammadHangup(call *CallInformation, cause string) error {
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

	resp, err := z.ClientZammad.Post(z.Config.Zammad.Endpoint, "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		return fmt.Errorf("unable to make request: %w", err)
	}

	if resp.StatusCode >= 300 {
		data, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("unexpected response (HTTP %d): %s", resp.StatusCode, string(data))
	}

	return nil
}
