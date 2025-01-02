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
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/rs/zerolog/log"
)

type WebsocketEventType int
type WebsocketResponse struct {
	Sequence int `json:"sequence"`
	Event    struct {
		EventType    WebsocketEventType `json:"event_type"`
		Entity       string             `json:"entity"`
		AttachedData *json.RawMessage   `json:"attached_data"`
	} `json:"event"`
}

const (
	WebsocketEventTypeUpsert     WebsocketEventType = 0
	WebsocketEventTypeDelete     WebsocketEventType = 1
	WebsocketEventTypeDTMFstring WebsocketEventType = 2
	WebsocketEventTypeResponse   WebsocketEventType = 4
)

type CallParticipant struct {
	ID int `json:"id"`

	// Status is the status of the call. Possible values include: "Dialing", "Ringing",
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

	client          http.Client
	phoneExtensions map[string]struct{}

	// accessToken is a Bearer-token retrieved after a valid Authentication call. It will expire automatically.
	accessToken string

	ongoingCalls map[string]CallInformation
}

func (z *Client3CXPost20) FetchCalls() ([]CallInformation, error) {
	// If we call someone, we get some entity like this:
	// {"level":"debug","entity":"{\"id\":8107,\"status\":\"Dialing\",\"dn\":\"150\",\"party_caller_name\":\"\",\"party_dn\":\"10007\",\"party_caller_id\":\"0123456789\",\"party_did\":\"\",\"device_id\":\"sip:150@127.0.0.1:5063\",\"party_dn_type\":\"Wexternalline\",\"direct_control\":false,\"originated_by_dn\":\"\",\"originated_by_type\":\"None\",\"referred_by_dn\":\"\",\"referred_by_type\":\"None\",\"on_behalf_of_dn\":\"\",\"on_behalf_of_type\":\"None\",\"callid\":1265,\"legid\":1}","sequence":18,"event_type":0,"time":"2024-12-30T15:12:40+01:00","message":"Received from 3CX WS"}
	// If we receive a call, we get some entity like this:
	// {"level":"debug","entity":"{\"id\":8106,\"status\":\"Ringing\",\"dn\":\"150\",\"party_caller_name\":\"+49123456789\",\"party_dn\":\"10007\",\"party_caller_id\":\"0123456789\",\"party_did\":\"\",\"device_id\":\"sip:150@127.0.0.1:5483;rinstance=c2a75fd2f1caea71\",\"party_dn_type\":\"Wexternalline\",\"direct_control\":false,\"originated_by_dn\":\"ROUTER\",\"originated_by_type\":\"Wroutepoint\",\"referred_by_dn\":\"\",\"referred_by_type\":\"None\",\"on_behalf_of_dn\":\"\",\"on_behalf_of_type\":\"None\",\"callid\":1264,\"legid\":4}","sequence":15,"event_type":0,"time":"2024-12-30T15:11:48+01:00","message":"Received from 3CX WS"}

	values := url.Values{}
	req, err := http.NewRequest("GET", z.Config.Phone3CX.Host+"/callcontrol?"+values.Encode(), nil)
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

	return z.aggregateCallResponse(callControlResponse), nil
}

func (z *Client3CXPost20) convertParticipant(participant CallParticipant, dn string) CallInformation {
	return CallInformation{
		Id:      json.Number(fmt.Sprintf("%d", participant.ID)),
		CallUID: fmt.Sprintf("%d", participant.CallID),

		Status:       participant.Status,
		CallerNumber: participant.PartyCallerID,
		CallerName:   "(" + participant.PartyCallerName + ")", // This would now be of the format "(+491234567890)"
		CalleeNumber: participant.DN,
		CalleeName:   "",
		AgentNumber:  dn,
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

// listenWS makes a Websocket connection to 3CX to "immediately" get updates on calls. This is a blocking function.
//
// Currently not used because the "immediate" updates aren't yet compatible with the current implementation.
func (z *Client3CXPost20) listenWS() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)
	defer signal.Stop(sigs)
	defer close(sigs)

	// Start a WS connection
	ctx := context.Background()

	c, _, err := websocket.Dial(ctx, z.Config.Phone3CX.Host+"/callcontrol/ws", &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Authorization": []string{"Bearer " + z.accessToken},
		},
	})
	if err != nil {
		log.Error().Err(err).Msg("Unable to connect to 3CX WS")
		return
	}

	defer c.Close(websocket.StatusNormalClosure, "")

	log.Debug().Msg("Connected to 3CX WS")

	go func() {
		for {
			select {
			case <-sigs:
				log.Debug().Msg("Received interrupt signal")
				_ = c.Close(websocket.StatusNormalClosure, "")
				os.Exit(0)
				return
			case <-ctx.Done():
				log.Debug().Msg("Context done")
				return
			}
		}
	}()

	// Make the initial request
	payload, err := json.Marshal(map[string]interface{}{
		"RequestID":   "123",
		"Path":        "/callcontrol",
		"RequestData": "",
	})
	if err != nil {
		log.Error().Err(err).Msg("Error marshalling initial request")
		return
	}

	err = c.Write(ctx, websocket.MessageText, payload)
	if err != nil {
		log.Error().Err(err).Msg("Error writing to 3CX WS")
		return
	}

	for {
		log.Trace().Msg("Waiting for data from 3CX WS...")
		_, data, err := c.Read(ctx)
		if err != nil {
			if errors.Is(err, io.EOF) {
				// Somehow the connection was closed
				log.Debug().Msg("WS Connection closed - received EOF")
				return
			}

			log.Error().Err(err).Msg("Error reading from 3CX WS")
			return
		}

		err = z.processWSMessage(data)
		if err != nil {
			log.Error().Err(err).Msg("Error processing WS message")
			continue
		}

		// TODO: Remove this
		//_ = z.fetchExtensions()
	}
}

func (z *Client3CXPost20) processWSMessage(msg []byte) error {
	var response WebsocketResponse
	err := json.Unmarshal(msg, &response)
	if err != nil {
		return fmt.Errorf("unable to parse WS message: %w", err)
	}

	if response.Event.EventType == WebsocketEventTypeDelete {
		log.Debug().
			Int("sequence", response.Sequence).
			Int("event_type", int(response.Event.EventType)).
			Msg("Received delete event from 3CX WS")
		// TODO: Delete from map
		return nil
	}

	// Fetch the entity data which includes current Status
	entityData, err := httpGET3CX[CallParticipant](z, z.Config.Phone3CX.Host+response.Event.Entity)
	if err != nil {
		return fmt.Errorf("unable to fetch entity data: %w", err)
	}

	log.Debug().
		Interface("entity", entityData).
		Int("sequence", response.Sequence).
		Int("event_type", int(response.Event.EventType)).
		Msg("Received from 3CX WS")

	// TODO: Store call in local map?

	return nil
}

// fetchExtensions fetches the details on group members of the 3CX group that we are monitoring.
func (z *Client3CXPost20) fetchExtensions() error {
	return nil // TODO remove me
}

func httpGET3CX[T any](z *Client3CXPost20, url string) (*T, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to prepare HTTP request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+z.accessToken)

	resp, err := z.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unable to perform HTTP request: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected response fetching 3CX info (HTTP %d): %s", resp.StatusCode, string(data))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to read response body: %w", err)
	}

	o := new(T)
	err = json.Unmarshal(respBody, o)
	if err != nil {
		return nil, fmt.Errorf("unable to parse response JSON: %w", err)
	}

	return o, nil
}

func (z *Client3CXPost20) getLoginValues() (url.Values, *http.Cookie, error) {
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

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to serialize JSON request body: %w", err)
	}

	resp, err := z.client.Post(z.Config.Phone3CX.Host+"/webclient/api/Login/GetAccessToken", "application/json", bytes.NewBuffer(jsonData))
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

	return values, &http.Cookie{
		Name:     "RefreshTokenCookie",
		Value:    loginResponse.Token.RefreshToken,
		Secure:   true,
		HttpOnly: true,
	}, nil
}

// Authenticate attempts to login to 3CX and stores a token for future API calls. It then loads
// all extensions we are configured to monitor.
func (z *Client3CXPost20) Authenticate() error {
	values, cookie, err := z.getLoginValues()
	if err != nil {
		return fmt.Errorf("unable to prepare login request: %w", err)
	}

	encodedPayload := values.Encode()

	req, err := http.NewRequest("POST", z.Config.Phone3CX.Host+"/connect/token?"+values.Encode(), bytes.NewReader([]byte(encodedPayload)))
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

	var tokenResponse struct {
		AccessToken string `json:"access_token"`
	}

	// Decode the response body into the tokenResponse struct
	err = json.NewDecoder(resp2.Body).Decode(&tokenResponse)
	if err != nil {
		panic(err)
	}

	z.accessToken = tokenResponse.AccessToken

	log.Debug().Msg("Successfully authenticated to 3CX")

	return z.fetchExtensions()
}

// AuthenticateRetry retries logging in a while (defined in maxOffline).
// It waits five seconds for every failed attempt.
func (z *Client3CXPost20) AuthenticateRetry(maxOffline time.Duration) error {
	var downSince time.Time

	for err := z.Authenticate(); err != nil; err = z.Authenticate() {
		// If we received a HTTP 404 error, the server is probably not online and might be a pre-v20 version.
		// Retrying will not help in such scenarios.
		if strings.Contains(err.Error(), "404") {
			return err
		}

		// Write down the start time
		if downSince.IsZero() {
			downSince = time.Now()
		}

		if time.Now().Sub(downSince) > maxOffline {
			return fmt.Errorf("unable to authenticate to 3CX: %w", err)
		}

		log.Warn().
			Err(err).
			Msg("Unable to authenticate to 3CX - retrying in 5 seconds...")
		time.Sleep(time.Second * 5)
	}

	return nil
}

func (z *Client3CXPost20) IsExtension(number string) bool {
	_, ok := z.phoneExtensions[number]
	return ok
}
