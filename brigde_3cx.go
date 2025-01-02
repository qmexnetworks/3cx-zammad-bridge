package zammadbridge

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

type CallInformation struct {
	Id     json.Number `json:"Id"`
	Caller string      `json:"Caller"`
	Callee string      `json:"Callee"`

	// Status has possible values: "Talking", "Transferring", "Routing"
	Status            string `json:"Status"`
	ZammadInitialized bool
	ZammadAnswered    bool

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
	Members struct {
		Selected []struct {
			Number struct {
				Value string `json:"_value"`
			} `json:"Number"`
		} `json:"selected"`
	} `json:"Members"`
}

// API3CX abstracts away the differences in API versions from before v20 and after v20 of 3CX.
type API3CX interface {
	// AuthenticateRetry authenticates the client and retries in case of offline status
	// maxOffline specifies the maximum duration to wait for the client to come online.
	AuthenticateRetry(maxOffline time.Duration) error

	// FetchCalls retrieves information about current calls from the 3CX API.
	// It returns a slice of CallInformation structs and an error if the API call fails.
	FetchCalls() ([]CallInformation, error)

	// IsExtension checks if a given phone number is a valid extension that is being monitored.
	IsExtension(number string) bool
}

// Create3CXClient creates a 3CX client based on the provided configuration.
//
// It first attempts to create a v20 client, and if it fails with an HTTP 404 error,
// it falls back to creating a pre-v20 client.
// The function returns an API3CX interface and an error if the client creation fails.
//
// The client is created with a cookiejar and authenticated using the AuthenticateRetry method,
// which waits for the client to come online for a maximum duration of two minutes.
func Create3CXClient(c *Config) (API3CX, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("unable to create cookiejar: %w", err)
	}

	// Try creating a v20 client, and if it fails with HTTP 404, we fallback to pre-v20
	v20 := &Client3CXPost20{
		Config: c,
		client: http.Client{
			Jar: jar,
		},
	}

	err = v20.AuthenticateRetry(120 * time.Second)
	if err == nil {
		return v20, nil
	}

	if !strings.Contains(err.Error(), "404") {
		return nil, err
	}

	log.Warn().
		Err(err).
		Msg("Falling back to 3CX (legacy) API client due to HTTP 404 error")

	preV20 := &Client3CXPre20{
		Config: c,
		client: http.Client{
			Jar: jar,
		},
	}

	err = preV20.AuthenticateRetry(120 * time.Second)
	if err != nil {
		return nil, err
	}

	return preV20, nil
}
