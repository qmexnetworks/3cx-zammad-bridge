package zammadbridge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"time"
)

type ZammadBridge struct {
	Config *Config

	Client3CX http.Client
	ClientZammad http.Client

	failedLogins int
}

// NewZammadBridge initializes a new client that listens for 3CX calls and forwards to Zammad.
func NewZammadBridge(config *Config) (*ZammadBridge, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("unable to create cookiejar: %w", err)
	}

	return &ZammadBridge{
		Config: config,
		Client3CX: http.Client{
			Jar: jar,
		},
	}, nil
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

	resp, err := z.Client3CX.Post(z.Config.Phone3CX.Host + "/api/login", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("unable to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		data, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("unexpected response: %s", string(data))
	}

	z.failedLogins = 0

	return nil
}

// Authenticate3CXRetries retries logging in a number of times to be more resilient.
// It waits one second for every failed attempt.
func (z *ZammadBridge) Authenticate3CXRetries(max int) error {
	for err := z.Authenticate3CX(); err != nil; err = z.Authenticate3CX() {
		z.failedLogins++

		// Give up
		if z.failedLogins >= max {
			return err
		}

		time.Sleep(time.Second * time.Duration(z.failedLogins))
	}

	return nil
}

// Listen listens for calls and does not return unless something really bad happened.
func (z *ZammadBridge) Listen() error {
	err := z.Authenticate3CX()
	if err != nil {
		return fmt.Errorf("unable to authenticate: %w", err)
	}

	return fmt.Errorf("not yet implemented")
}
