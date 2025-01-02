package zammadbridge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

type Client3CXPre20 struct {
	Config *Config

	client          http.Client
	phoneExtensions map[string]struct{}
}

func (z *Client3CXPre20) FetchCalls() ([]CallInformation, error) {
	type CallInformationResponse struct {
		List []CallInformation `json:"list"`
	}

	resp, err := z.client.Get(z.Config.Phone3CX.Host + "/api/activeCalls")
	if err != nil {
		return nil, fmt.Errorf("unable to request from 3CX: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected response fetching the ongoing call list from 3CX (HTTP %d): %s", resp.StatusCode, string(data))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to read response body: %w", err)
	}

	var response CallInformationResponse
	err = json.Unmarshal(data, &response)
	if err != nil {
		return nil, fmt.Errorf("unable to parse JSON response: %w", err)
	}

	// Process names / numbers already
	for i := 0; i < len(response.List); i++ {
		callerInfo := strings.SplitN(response.List[i].Caller, " ", 2)
		calleeInfo := strings.SplitN(response.List[i].Callee, " ", 2)

		response.List[i].CallerNumber = callerInfo[0]
		if len(callerInfo) == 2 {
			response.List[i].CallerName = callerInfo[1]
		}
		response.List[i].CalleeNumber = calleeInfo[0]
		if len(calleeInfo) == 2 {
			response.List[i].CalleeName = calleeInfo[1]
		}
	}

	return response.List, nil
}

// fetchGroupMembers fetches the details on group members of the 3CX group that we are monitoring.
func (z *Client3CXPre20) fetchGroupMembers() error {
	// Request to /api/edit/update with complex payload
	groupId, count, err := z.fetchGroupId(z.Config.Phone3CX.Group)
	if err != nil {
		return fmt.Errorf("unable to find 3CX group id: %w", err)
	}

	_, objectId, err := z.fetchGroupMembersPageFirst(groupId)
	if err != nil {
		return fmt.Errorf("unable to fetch group object id %s: %w", groupId, err)
	}

	var startIndex = 0
	var allExtensions []string
	z.phoneExtensions = map[string]struct{}{}

	for len(z.phoneExtensions) < count && startIndex <= count {
		extensions, err := z.fetchGroupMembersPage(objectId, startIndex)
		if err != nil {
			return fmt.Errorf("unable to fetch group members from index %d from group %s: %w", startIndex, groupId, err)
		}

		for _, e := range extensions {
			z.phoneExtensions[e] = struct{}{}
			allExtensions = append(allExtensions, e)
		}

		if len(extensions) == 0 {
			break
		}

		startIndex += len(extensions)
	}

	log.Info().Interface("extensions", allExtensions).Msg("Loaded extensions")

	return nil
}

// fetchGroupMembersPageFirst fetches the first page of members of the given group
// TODO: For v20 and up we need to request /xapi/v1/Groups(ID)/Members and store the `.Number` field within `value` where they have `.Type = "RingGroup"`
func (z *Client3CXPre20) fetchGroupMembersPageFirst(groupId string) ([]string, string, error) {
	requestBody := fmt.Sprintf(
		"{\"Id\":%s}",
		groupId)

	resp, err := z.client.Post(z.Config.Phone3CX.Host+"/api/GroupList/set", "application/json", bytes.NewBufferString(requestBody))
	if err != nil {
		return nil, "", fmt.Errorf("unable to make GroupList/set request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("unexpected response fetching 3CX group membership assignments (HTTP %d): %s", resp.StatusCode, string(data))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("unable to read response body: %w", err)
	}

	var response struct {
		ActiveObject GroupListEntryObject `json:"ActiveObject"`
		Id           int                  `json:"Id"`
	}
	err = json.Unmarshal(respBody, &response)
	if err != nil {
		return nil, "", fmt.Errorf("unable to parse response JSON: %w", err)
	}

	var extensions []string
	for _, mem := range response.ActiveObject.Members.Selected {
		extensions = append(extensions, mem.Number.Value)
	}

	return extensions, strconv.Itoa(response.Id), nil
}

// fetchGroupMembersPage fetches a single page of members of the given group
func (z *Client3CXPre20) fetchGroupMembersPage(objectId string, start int) ([]string, error) {
	requestBody := fmt.Sprintf(
		"{\"Path\":{\"ObjectId\":\"%s\",\"PropertyPath\":[{\"Name\":\"Members\"}]},\"PropertyValue\":{\"State\":{\"Start\":%d,\"SortBy\":null,\"Reverse\":false,\"Search\":\"\"}}}",
		objectId, start)

	resp, err := z.client.Post(z.Config.Phone3CX.Host+"/api/edit/update", "application/json", bytes.NewBufferString(requestBody))
	if err != nil {
		return nil, fmt.Errorf("unable to make group member listing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected response fetching 3CX group membership assignments (HTTP %d): %s", resp.StatusCode, string(data))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to read response body: %w", err)
	}

	var entries []GroupListResponseEntry
	err = json.Unmarshal(respBody, &entries)
	if err != nil {
		return nil, fmt.Errorf("unable to parse response JSON: %w", err)
	}

	var extensions []string
	for _, e := range entries {
		for _, mem := range e.Item.Members.Selected {
			extensions = append(extensions, mem.Number.Value)
		}
	}

	return extensions, nil
}

// fetchGroupId looks for the internal 3CX id for the given group
func (z *Client3CXPre20) fetchGroupId(groupName string) (Id string, Count int, err error) {
	// Request to /api/GroupList and then look for the name
	resp, err := z.client.Get(z.Config.Phone3CX.Host + "/api/GroupList")
	if err != nil {
		err = fmt.Errorf("unable to request group list: %w", err)
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		err = fmt.Errorf("unexpected response fetching 3CX group info (HTTP %d): %s", resp.StatusCode, string(data))
		return
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		err = fmt.Errorf("unable to read response body: %w", err)
		return
	}

	var groupListResponse struct {
		List []struct {
			Id              int    `json:"Id"`
			Name            string `json:"Name"`
			ExtensionsCount int    `json:"ExtensionsCount"`
		} `json:"list"`
	}
	err = json.Unmarshal(respBody, &groupListResponse)
	if err != nil {
		err = fmt.Errorf("unable to parse response JSON: %w", err)
		return
	}

	for _, group := range groupListResponse.List {
		if group.Name == groupName {
			return strconv.Itoa(group.Id), group.ExtensionsCount, nil
		}
	}

	err = fmt.Errorf("group by name not found: %q", groupName)
	return
}

// Authenticate attempts to login to 3CX and retrieve a valid cookie session.
func (z *Client3CXPre20) Authenticate() error {
	log.Debug().
		Str("host", z.Config.Phone3CX.Host).
		Str("user", z.Config.Phone3CX.User).
		Msg("Authenticating to 3CX (legacy)...")

	requestBody := struct {
		Username string
		Password string
	}{
		z.Config.Phone3CX.User,
		z.Config.Phone3CX.Pass,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("unable to serialize JSON request body: %w", err)
	}

	resp, err := z.client.Post(z.Config.Phone3CX.Host+"/api/login", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("unable to make login request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected response authenticating 3CX (HTTP %d): %s", resp.StatusCode, string(data))
	}

	return z.fetchGroupMembers()
}

// AuthenticateRetry retries logging in a while (defined in maxOffline).
// It waits five seconds for every failed attempt.
func (z *Client3CXPre20) AuthenticateRetry(maxOffline time.Duration) error {
	var downSince time.Time

	for err := z.Authenticate(); err != nil; err = z.Authenticate() {
		// Write down the start time
		if downSince.IsZero() {
			downSince = time.Now()
		}

		if time.Now().Sub(downSince) > maxOffline {
			return fmt.Errorf("unable to authenticate to 3CX (legacy): %w", err)
		}

		log.Warn().
			Err(err).
			Msg("Unable to authenticate to 3CX (legacy) - retrying in 5 seconds...")
		time.Sleep(time.Second * 5)
	}

	return nil
}

func (z *Client3CXPre20) IsExtension(number string) bool {
	_, ok := z.phoneExtensions[number]
	return ok
}
