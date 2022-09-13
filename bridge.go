package zammadbridge

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"strconv"
	"strings"
	"time"

	uuid "github.com/satori/go.uuid"
)

type ZammadBridge struct {
	Config *Config

	Client3CX    http.Client
	ClientZammad http.Client

	phoneExtensions map[string]struct{}
	ongoingCalls    map[json.Number]CallInformation
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
		ongoingCalls: map[json.Number]CallInformation{},
	}, nil
}

// Listen listens for calls and does not return unless something really bad happened.
func (z *ZammadBridge) Listen() error {
	err := z.Authenticate3CXRetries(time.Second * 120)
	if err != nil {
		return fmt.Errorf("unable to authenticate: %w", err)
	}

	for {
		err = z.RequestAndProcess()
		if err != nil && (strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "403")) {
			StdVerbose.Println("Reconnecting due to", err.Error())

			// Authentication error
			err = z.Authenticate3CXRetries(time.Second * 120)
			if err != nil {
				return fmt.Errorf("unable to authenticate: %w", err)
			}
		} else if err != nil {
			StdErr.Println("Error", err.Error())
		}

		// Wait until the next polling should occur
		time.Sleep(time.Duration(float64(time.Second) * z.Config.Bridge.PollInterval))
	}
}

// RequestAndProcess requests the current calls from 3CX and processes them to Zammad
func (z *ZammadBridge) RequestAndProcess() error {
	calls, err := z.Fetch3CXCalls()
	newCalls := make([]json.Number, 0, len(calls))
	for _, c := range calls {
		err = z.ProcessCall(&c)
		if err != nil {
			StdErr.Println("Warning - error processing call", err.Error())
		}

		newCalls = append(newCalls, c.Id)
	}

	var endedCalls []json.Number

endedCallLoop:
	for callId, oldInfo := range z.ongoingCalls {
		// Check if call is still ongoing
		for _, newCallId := range newCalls {
			if newCallId == callId {
				continue endedCallLoop
			}
		}

		// Apparently the call has ended, because 3CX does not report it any longer
		StdVerbose.Printf("Call with 3CX-ID %s and Zammad-ID %s not reported by 3CX, assuming call ended", callId, oldInfo.CallUID)
		endedCalls = append(endedCalls, callId)
		if oldInfo.Status == "Routing" {
			StdOut.Printf("Call with ID %s %s was not answered", oldInfo.CallUID, oldInfo.Direction)
			z.LogIfErr(z.ZammadHangup(&oldInfo, "cancel"), "hangup-from-routing")
		} else if oldInfo.Status == "Talking" {
			StdOut.Printf("Call with ID %s %s was hangup", oldInfo.CallUID, oldInfo.Direction)
			z.LogIfErr(z.ZammadHangup(&oldInfo, "normalClearing"), "hangup-from-talking")
		} else if oldInfo.Status == "Transferring" && z.Config.Zammad.LogMissedQueueCalls {
			StdOut.Printf("Queue call with ID %s %s was not answered", oldInfo.CallUID, oldInfo.Direction)
			oldInfo.AgentNumber = strconv.Itoa(z.Config.Phone3CX.QueueExtension)
			z.LogIfErr(z.ZammadHangup(&oldInfo, "cancel"), "hangup-from-transferring")
		}
	}

	for _, callId := range endedCalls {
		delete(z.ongoingCalls, callId)
	}

	return err
}

// ProcessCall processes a single ongoing call from 3CX
func (z *ZammadBridge) ProcessCall(call *CallInformation) error {
	if z.isOutboundCall(call) {
		call.Direction = "Outbound"
		call.AgentNumber = call.CallerNumber
		call.AgentName = call.CallerName
		call.ExternalNumber = z.ParsePhoneNumber(call.CalleeNumber + " " + call.CalleeName)
		call.CallTo = call.ExternalNumber
		call.CallFrom = call.AgentNumber
	} else if z.isInboundCall(call) {
		call.Direction = "Inbound"
		call.AgentNumber = call.CalleeNumber
		call.AgentName = call.CalleeName
		call.ExternalNumber = z.ParsePhoneNumber(call.CallerNumber + " " + call.CallerName)
		call.CallTo = call.AgentNumber
		call.CallFrom = call.ExternalNumber
	} else {
		return nil
	}

	if z.isNewCall(call) {
		// Save it for the first time
		call.CallUID = uuid.NewV4().String()

		// Notify all active Zammad clients that someone is calling
		StdOut.Printf("New call (%s) with ID %s %s from %s to %s", call.Status, call.CallUID, call.Direction, call.CallFrom, call.CallTo)
		z.LogIfErr(z.ZammadNewCall(call), "new-call")
	} else {
		// Update call information
		previous := z.ongoingCalls[call.Id]
		call.CallUID = previous.CallUID
		call.ZammadInitialized = previous.ZammadInitialized
		call.ZammadAnswered = previous.ZammadAnswered

		// If the call is now "Talking", it means we are currently talking to someone. It is with someone of our loaded
		// extensions due to the early-return that otherwise would have happened.
		// We should then, for once, let Zammad know we answered this call. Since the "Talking" status can be present
		// every tick, we need to check if we already notified Zammad and only notify Zammad as-needed.
		if call.Status == "Talking" {
			if !previous.ZammadAnswered {
				StdOut.Printf("Call with ID %s %s from %s was answered by %s", call.CallUID, call.Direction, call.CallFrom, call.CallTo)
				z.LogIfErr(z.ZammadAnswer(call), "answer")
			}
		}
	}

	z.ongoingCalls[call.Id] = *call
	return nil
}

// isInboundCall checks whether the given call is an inbound call.
func (z *ZammadBridge) isInboundCall(call *CallInformation) bool {
	if len(call.CallerNumber) != z.Config.Phone3CX.TrunkDigits {
		return false
	}

	if len(call.CalleeNumber) != z.Config.Phone3CX.ExtensionDigits {
		return false
	}

	if _, ok := z.phoneExtensions[call.CalleeNumber]; !ok {
		return false
	}

	return true
}

// isOutboundCall checks whether the given call is an outbound call.
func (z *ZammadBridge) isOutboundCall(call *CallInformation) bool {
	if len(call.CallerNumber) != z.Config.Phone3CX.ExtensionDigits {
		return false
	}

	if len(call.CalleeNumber) != z.Config.Phone3CX.TrunkDigits {
		return false
	}

	if _, ok := z.phoneExtensions[call.CallerNumber]; !ok {
		return false
	}

	return true
}

// isNewCall checks whether the given call is already ongoing and previously detected by the bridge.
func (z *ZammadBridge) isNewCall(call *CallInformation) bool {
	_, ok := z.ongoingCalls[call.Id]
	return !ok
}

// ParsePhoneNumber parses the phone number into a format acceptable to Zammad
func (z *ZammadBridge) ParsePhoneNumber(number string) string {
	// Number is between two brackets, e.g. (0123)
	if strings.Contains(number, "(") {
		number = number[strings.Index(number, "(")+1 : strings.LastIndex(number, ")")]
	}

	// TODO what if +49/Germany isn't the default? See 3CX settings.
	// old filter, should better be done in 3CX E.164
	if strings.HasPrefix(number, "+49") {
		number = "0" + number[3:]
	} else if strings.HasPrefix(number, "49") {
		number = "0" + number[2:]
	}

	return number
}

// LogIfErr logs to stderr when an error occurs, doing nothing when err is nil.
func (z *ZammadBridge) LogIfErr(err error, context string) {
	if err == nil {
		return
	}

	StdErr.Println("Error", context, err.Error())
}
