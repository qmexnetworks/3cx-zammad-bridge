package zammadbridge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

type CallParticipant struct {
	ID int `json:"id"`

	// Status is the status of the call. Possible values include: "Dialing", "Ringing", "Connected"
	Status string `json:"status"`

	// DN is the extension number of the participant.
	DN string `json:"dn"`

	// PartyCallerName is the name of the caller or callee. Can be empty.
	PartyCallerName string `json:"party_caller_name"`

	// PartyDN is the extension number of the caller. E.g. 10007
	PartyDN string `json:"party_dn"`

	// PartyCallerID is the caller ID of the caller. E.g. 0123456789
	PartyCallerID string `json:"party_caller_id"`

	// PartyDID is the DID of the caller. Can be empty.
	PartyDID string `json:"party_did"`

	// CallID is the unique ID of the call.
	CallID int `json:"callid"`
}

type CallControlResponse []CallControlResponseEntry

type CallControlResponseEntry struct {
	DN           string            `json:"dn"`
	Participants []CallParticipant `json:"participants"`
}

type Client3CXPost20 struct {
	Config *Config

	client http.Client

	// accessToken is a Bearer-token retrieved after a valid Authentication call. It will expire automatically.
	accessToken string
}

func (z *Client3CXPost20) FetchCalls(ctx context.Context) ([]CallInformation, error) {
	// If we call someone, we get some entity like this:
	// {"level":"debug","entity":"{\"id\":8107,\"status\":\"Dialing\",\"dn\":\"150\",\"party_caller_name\":\"\",\"party_dn\":\"10007\",\"party_caller_id\":\"0123456789\",\"party_did\":\"\",\"device_id\":\"sip:150@127.0.0.1:5063\",\"party_dn_type\":\"Wexternalline\",\"direct_control\":false,\"originated_by_dn\":\"\",\"originated_by_type\":\"None\",\"referred_by_dn\":\"\",\"referred_by_type\":\"None\",\"on_behalf_of_dn\":\"\",\"on_behalf_of_type\":\"None\",\"callid\":1265,\"legid\":1}","sequence":18,"event_type":0,"time":"2024-12-30T15:12:40+01:00","message":"Received from 3CX WS"}
	// If we receive a call, we get some entity like this:
	// {"level":"debug","entity":"{\"id\":8106,\"status\":\"Ringing\",\"dn\":\"150\",\"party_caller_name\":\"+49123456789\",\"party_dn\":\"10007\",\"party_caller_id\":\"0123456789\",\"party_did\":\"\",\"device_id\":\"sip:150@127.0.0.1:5483;rinstance=c2a75fd2f1caea71\",\"party_dn_type\":\"Wexternalline\",\"direct_control\":false,\"originated_by_dn\":\"ROUTER\",\"originated_by_type\":\"Wroutepoint\",\"referred_by_dn\":\"\",\"referred_by_type\":\"None\",\"on_behalf_of_dn\":\"\",\"on_behalf_of_type\":\"None\",\"callid\":1264,\"legid\":4}","sequence":15,"event_type":0,"time":"2024-12-30T15:11:48+01:00","message":"Received from 3CX WS"}

	values := url.Values{}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, z.Config.Phone3CX.Host+"/callcontrol?"+values.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("unable to prepare HTTP request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+z.accessToken)

	// Request to /api/GroupList and then look for the name
	resp, err := z.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unable to request group list: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		log.Debug().
			Str("response", string(data)).
			Interface("headers", resp.Header).
			Msg("Received group list response")
		return nil, fmt.Errorf("unexpected response fetching 3CX group info (HTTP %d): %s", resp.StatusCode, string(data))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to read response body: %w", err)
	}

	var callControlResponse CallControlResponse
	err = json.Unmarshal(respBody, &callControlResponse)
	if err != nil {
		return nil, fmt.Errorf("unable to parse response JSON: %w", err)
	}

	log.Trace().
		Interface("response", callControlResponse).
		Msg("Received call control response")

	return z.aggregateCallResponse(callControlResponse), nil
}

func (z *Client3CXPost20) convertParticipant(participant CallParticipant, dn string) CallInformation {
	if participant.Status == "Connected" {
		participant.Status = statusTalking // This is the pre v20 status
	}

	return CallInformation{
		ID: json.Number(strconv.Itoa(participant.CallID)),
		// CallUID: strconv.Itoa(participant.CallID),

		Status:       participant.Status,
		CallerNumber: participant.PartyDN,
		CallerName:   participant.PartyCallerName + " (" + participant.PartyCallerID + ")", // This would now be of the format "(+491234567890)"
		CalleeNumber: participant.DN,
		CalleeName:   "",
		AgentNumber:  dn,

		LastChangeStatus: time.Now(),
		EstablishedAt:    time.Now(), // Not quite correct ...
	}
}

func (z *Client3CXPost20) aggregateCallResponse(response CallControlResponse) []CallInformation {
	var calls []CallInformation
	for _, entry := range response {
		for _, participant := range entry.Participants {
			calls = append(calls, z.convertParticipant(participant, entry.DN))
		}
	}

	return calls
}

func (z *Client3CXPost20) getLoginValues(ctx context.Context) (url.Values, *http.Cookie, error) {
	if z.Config.Phone3CX.ClientID != "" && z.Config.Phone3CX.ClientSecret != "" {
		log.Debug().
			Str("client_id", z.Config.Phone3CX.ClientID).
			Msgf("Authenticating to 3CX...")
		return url.Values{
			"grant_type":    {"client_credentials"},
			"client_id":     {z.Config.Phone3CX.ClientID},
			"client_secret": {z.Config.Phone3CX.ClientSecret},
		}, nil, nil
	}

	requestBody := struct {
		Username     string `json:"Username"`
		Password     string `json:"Password"`
		SecurityCode string `json:"SecurityCode"`
	}{
		z.Config.Phone3CX.User,
		z.Config.Phone3CX.Pass,
		"",
	}

	log.Debug().
		Str("host", z.Config.Phone3CX.Host).
		Str("user", z.Config.Phone3CX.User).
		Msgf("Authenticating to 3CX...")

	jsonData, err := json.Marshal(requestBody) //nolint:gosec // G117: Password is the 3CX login payload
	if err != nil {
		return nil, nil, fmt.Errorf("unable to serialize JSON request body: %w", err)
	}

	resp, err := httpPostJSON(ctx, &z.client, z.Config.Phone3CX.Host+"/webclient/api/Login/GetAccessToken", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, nil, fmt.Errorf("unable to make login request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return nil, nil, fmt.Errorf("unexpected response authenticating 3CX (HTTP %d): %s", resp.StatusCode, string(data))
	}

	var loginResponse struct {
		Status string `json:"Status"`
		Token  struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		} `json:"Token"`
	}

	// Decode the response body into the loginResponse struct
	err = json.NewDecoder(resp.Body).Decode(&loginResponse)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to decode login response: %w", err)
	}

	if loginResponse.Status != "AuthSuccess" {
		return nil, nil, fmt.Errorf("login failed: %s", loginResponse.Status)
	}
	values := url.Values{}
	values.Add("grant_type", "refresh_token")
	values.Add("client_id", "Webclient")

	return values, &http.Cookie{ //nolint:gosec // G124: outgoing cookie sent to 3CX, already Secure
		Name:     "RefreshTokenCookie",
		Value:    loginResponse.Token.RefreshToken,
		Secure:   true,
		HttpOnly: true,
	}, nil
}

// Authenticate attempts to login to 3CX and stores a token for future API calls. It then loads
// all extensions we are configured to monitor.
func (z *Client3CXPost20) Authenticate(ctx context.Context) error {
	values, cookie, err := z.getLoginValues(ctx)
	if err != nil {
		return fmt.Errorf("unable to prepare login request: %w", err)
	}

	// The credentials only go in the body: errors include the URL, and those end up in the logs.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, z.Config.Phone3CX.Host+"/connect/token", strings.NewReader(values.Encode()))
	if err != nil {
		return fmt.Errorf("unable to prepare HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=utf-8")

	if cookie != nil {
		req.AddCookie(cookie)
	}

	resp2, err := z.client.Do(req)
	if err != nil {
		return fmt.Errorf("unable to perform HTTP request: %w", err)
	}
	defer resp2.Body.Close()

	var tokenResponse struct {
		AccessToken string `json:"access_token"`
	}

	respBody, err := io.ReadAll(resp2.Body)
	if err != nil {
		return fmt.Errorf("unable to read response body: %w", err)
	}

	if resp2.StatusCode == http.StatusUnauthorized {
		return errors.New("HTTP 401: using wrong client_id or client_secret?")
	}

	log.Trace().
		Int("http_status", resp2.StatusCode).
		Int("response_length", len(respBody)).
		Msg("Received response from 3CX")

	// Decode the response body into the tokenResponse struct
	err = json.Unmarshal(respBody, &tokenResponse)
	if err != nil {
		return fmt.Errorf("unable to unmarshal access token: %w", err)
	}

	z.accessToken = tokenResponse.AccessToken

	log.Debug().Msg("Successfully authenticated to 3CX")

	return nil
}

// AuthenticateRetry retries logging in a while (defined in maxOffline).
// It waits five seconds for every failed attempt.
func (z *Client3CXPost20) AuthenticateRetry(ctx context.Context, maxOffline time.Duration) error {
	var downSince time.Time

	for err := z.Authenticate(ctx); err != nil; err = z.Authenticate(ctx) {
		// If we received a HTTP 404 error, the server is probably not online and might be a pre-v20 version.
		// Retrying will not help in such scenarios.
		if strings.Contains(err.Error(), "404") {
			return err
		}

		// Write down the start time
		if downSince.IsZero() {
			downSince = time.Now()
		}

		if time.Since(downSince) > maxOffline {
			return fmt.Errorf("unable to authenticate to 3CX: %w", err)
		}

		log.Warn().
			Err(err).
			Msg("Unable to authenticate to 3CX - retrying in 5 seconds...")
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}

	return nil
}

func (z *Client3CXPost20) IsExtension(_ string) bool {
	// In v20 and above, we are only shown the extensions we are monitoring.
	// Therefore, we can assume that if we are monitoring an extension, it is valid.
	return true
}
