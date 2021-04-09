package zammadbridge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"strconv"
	"strings"
	"time"
)

type CallInformation struct {
	Id     json.Number `json:"Id"`
	Caller string `json:"Caller"`
	Callee string `json:"Callee"`

	// Status has possible values: "Talking", "Transferring", "Routing"
	Status string `json:"Status"`

	// Various processed fields
	CallerName     string
	CallerNumber   string
	CalleeName     string
	CalleeNumber   string
	CallUID        string
	Direction      string
	AgentName      string
	AgentNumber    string
	CallFrom       string
	CallTo         string
	ExternalNumber string
}

type GroupListResponseEntry struct {
	Item GroupListEntryObject `json:"Item"`
}

type GroupListEntryObject struct {
	Members struct{
		Selected []struct{
			Number struct {
				Value string `json:"_value"`
			} `json:"Number"`
		} `json:"selected"`
	} `json:"Members"`
}

func (z *ZammadBridge) Fetch3CXCalls() ([]CallInformation, error) {
	type CallInformationResponse struct {
		List []CallInformation `json:"list"`
	}

	resp, err := z.Client3CX.Get(z.Config.Phone3CX.Host + "/api/activeCalls")
	if err != nil {
		return nil, fmt.Errorf("unable to request from 3CX: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		data, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected response (HTTP %d): %s", resp.StatusCode, string(data))
	}

	data, err := ioutil.ReadAll(resp.Body)
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

//Fetch3CXGroupMembers fetches the details on group members of the 3CX group that we are monitoring.
func (z *ZammadBridge) Fetch3CXGroupMembers() error {
	// Request to /api/edit/update with complex payload
	groupId, count, err := z.Fetch3CXGroupId(z.Config.Phone3CX.Group)
	if err != nil {
		return fmt.Errorf("unable to find 3CX group id: %w", err)
	}

	var startIndex = 0
	var objectId string
	var allExtensions []string
	z.phoneExtensions = map[string]struct{}{}

	for len(z.phoneExtensions) < count && startIndex <= count {
		var extensions []string
		if startIndex == 0 {
			extensions, objectId, err = z.Fetch3CXGroupMembersPageFirst(groupId)
		} else {
			extensions, err = z.Fetch3CXGroupMembersPage(objectId, startIndex)
		}
		if err != nil {
			return fmt.Errorf("unable to fetch group members from index %d from group %s: %w", startIndex, groupId, err)
		}

		for _, e := range extensions {
			z.phoneExtensions[e] = struct{}{}
			allExtensions = append(allExtensions, e)
		}

		startIndex += len(extensions)
	}

	StdOut.Println("Loaded extensions:", allExtensions)

	return nil
}

// Fetch3CXGroupMembersPageFirst fetches the first page of members of the given group
func (z *ZammadBridge) Fetch3CXGroupMembersPageFirst(groupId string) ([]string, string, error) {
	requestBody := fmt.Sprintf(
		"{\"Id\":%s}",
		groupId)

	resp, err := z.Client3CX.Post(z.Config.Phone3CX.Host + "/api/GroupList/set", "application/json", bytes.NewBufferString(requestBody))
	if err != nil {
		return nil,  "", fmt.Errorf("unable to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		data, _ := ioutil.ReadAll(resp.Body)
		return nil, "",  fmt.Errorf("unexpected response (HTTP %d): %s", resp.StatusCode, string(data))
	}

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, "",  fmt.Errorf("unable to read response body: %w", err)
	}

	var response struct {
		ActiveObject GroupListEntryObject `json:"ActiveObject"`
		Id int `json:"Id"`
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

// Fetch3CXGroupMembersPage fetches a single page of members of the given group
func (z *ZammadBridge) Fetch3CXGroupMembersPage(objectId string, start int) ([]string, error) {
	requestBody := fmt.Sprintf(
		"{\"Path\":{\"ObjectId\":\"%s\",\"PropertyPath\":[{\"Name\":\"Members\"}]},\"PropertyValue\":{\"State\":{\"Start\":%d,\"SortBy\":null,\"Reverse\":false,\"Search\":\"\"}}}",
		objectId, start)

	resp, err := z.Client3CX.Post(z.Config.Phone3CX.Host + "/api/edit/update", "application/json", bytes.NewBufferString(requestBody))
	if err != nil {
		return nil, fmt.Errorf("unable to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		data, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected response (HTTP %d): %s", resp.StatusCode, string(data))
	}

	respBody, err := ioutil.ReadAll(resp.Body)
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

// Fetch3CXGroupId looks for the internal 3CX id for the given group
func (z *ZammadBridge) Fetch3CXGroupId(groupName string) (Id string, Count int, err error) {
	// Request to /api/GroupList and then look for the name
	resp, err := z.Client3CX.Get(z.Config.Phone3CX.Host + "/api/GroupList")
	if err != nil {
		err = fmt.Errorf("unable to request group list: %w", err)
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		data, _ := ioutil.ReadAll(resp.Body)
		err =  fmt.Errorf("unexpected response (HTTP %d): %s", resp.StatusCode, string(data))
		return
	}

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		err = fmt.Errorf("unable to read response body: %w", err)
		return
	}

	var groupListResponse struct{
		List []struct{
			Id int `json:"Id"`
			Name string `json:"Name"`
			ExtensionsCount int `json:"ExtensionsCount"`
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

// Authenticate3CX attempts to login to 3CX and retrieve a valid cookie session.
func (z *ZammadBridge) Authenticate3CX() error {
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

	resp, err := z.Client3CX.Post(z.Config.Phone3CX.Host+"/api/login", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("unable to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		data, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("unexpected response (HTTP %d): %s", resp.StatusCode, string(data))
	}

	return z.Fetch3CXGroupMembers()
}

// Authenticate3CXRetries retries logging in a while (defined in maxOffline).
// It waits five seconds for every failed attempt.
func (z *ZammadBridge) Authenticate3CXRetries(maxOffline time.Duration) error {
	var downSince time.Time

	for err := z.Authenticate3CX(); err != nil; err = z.Authenticate3CX() {
		// Write down the start time
		if downSince.IsZero() {
			downSince = time.Now()
		}

		if time.Now().Sub(downSince) > maxOffline {
			return fmt.Errorf("unable to authenticate to 3CX: %w", err)
		}

		log.Printf("Unable to authenticate to 3CX (%s) - retrying in 5 seconds...", err.Error())
		time.Sleep(time.Second * 5)
	}

	return nil
}
