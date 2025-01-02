package zammadbridge

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

type ZammadBridge struct {
	Config *Config

	Client3CX    API3CX
	ClientZammad http.Client

	ongoingCalls map[json.Number]CallInformation
}

// NewZammadBridge initializes a new client that listens for 3CX calls and forwards to Zammad.
func NewZammadBridge(config *Config) (*ZammadBridge, error) {
	client3CX, err := Create3CXClient(config)
	if err != nil {
		return nil, fmt.Errorf("unable to create 3CX client: %w", err)
	}

	return &ZammadBridge{
		Config:       config,
		Client3CX:    client3CX,
		ongoingCalls: map[json.Number]CallInformation{},
	}, nil
}

// Listen listens for calls and does not return unless something really bad happened.
func (z *ZammadBridge) Listen() error {
	log.Info().Msg("Starting 3CX-Zammad bridge (fetching calls every " + strconv.FormatFloat(z.Config.Bridge.PollInterval, 'f', -1, 64) + " seconds)")
	for {
		err := z.RequestAndProcess()
		if err != nil && (strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "403")) {
			log.Trace().Err(err).Msg("Reconnecting due to authentication error")

			// Authentication error
			err = z.Client3CX.AuthenticateRetry(time.Second * 120)
			if err != nil {
				return fmt.Errorf("unable to authenticate: %w", err)
			}
		} else if err != nil {
			log.Error().Err(err).Msg("Error processing calls")
		}

		// Wait until the next polling should occur
		time.Sleep(time.Duration(float64(time.Second) * z.Config.Bridge.PollInterval))
	}
}

// RequestAndProcess requests the current calls from 3CX and processes them to Zammad
func (z *ZammadBridge) RequestAndProcess() error {
	calls, err := z.Client3CX.FetchCalls()
	newCalls := make([]json.Number, 0, len(calls))
	for _, c := range calls {
		err = z.ProcessCall(&c)
		if err != nil {
			log.Warn().Err(err).Msg("Warning - error processing call")
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

		// Apparently, the call has ended, because 3CX does not report it any longer
		log.Trace().Str("call_id", oldInfo.CallUID).Str("direction", oldInfo.Direction).Str("from", oldInfo.CallFrom).Str("to", oldInfo.CallTo).Msg("Call ended (no longer reported by 3CX)")
		endedCalls = append(endedCalls, callId)
		if oldInfo.Status == "Routing" {
			log.Info().Str("call_id", oldInfo.CallUID).Str("direction", oldInfo.Direction).Str("from", oldInfo.CallFrom).Str("to", oldInfo.CallTo).Msg("Call ended (hangup from routing)")
			z.LogIfErr(z.ZammadHangup(&oldInfo, "cancel"), "hangup-from-routing")
		} else if oldInfo.Status == "Talking" {
			log.Info().Str("call_id", oldInfo.CallUID).Str("direction", oldInfo.Direction).Str("from", oldInfo.CallFrom).Str("to", oldInfo.CallTo).Msg("Call ended (hangup from talking)")
			z.LogIfErr(z.ZammadHangup(&oldInfo, "normalClearing"), "hangup-from-talking")
		} else if oldInfo.Status == "Transferring" && z.Config.Zammad.LogMissedQueueCalls {
			log.Info().Str("call_id", oldInfo.CallUID).Str("direction", oldInfo.Direction).Str("from", oldInfo.CallFrom).Str("to", oldInfo.CallTo).Msg("Queue call was not answered")
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
		call.CallUID = uuid.New().String()

		// Notify all active Zammad clients that someone is calling
		log.Info().Str("call_id", call.CallUID).Str("direction", call.Direction).Str("from", call.CallFrom).Str("to", call.CallTo).Msg("New call")
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
				log.Info().Str("call_id", call.CallUID).Str("direction", call.Direction).Str("from", call.CallFrom).Str("to", call.CallTo).Msg("Call answered")
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

	if !z.Client3CX.IsExtension(call.CalleeNumber) {
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

	if !z.Client3CX.IsExtension(call.CallerNumber) {
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

	// If no prefix is configured, we cannot do anything
	if z.Config.Phone3CX.CountryPrefix == "" {
		return number
	}

	// If it starts with e.g., +49, we remove that prefix and add a 0 instead
	if strings.HasPrefix(number, "+"+z.Config.Phone3CX.CountryPrefix) {
		return "0" + number[len(z.Config.Phone3CX.CountryPrefix)+1:]
	}

	// If it starts with e.g., 0049, we remove that prefix and add a 0 instead
	if strings.HasPrefix(number, "00"+z.Config.Phone3CX.CountryPrefix) {
		return "0" + number[len(z.Config.Phone3CX.CountryPrefix)+2:]
	}

	// If it starts with e.g., 49, we remove that prefix and add a 0 instead
	if strings.HasPrefix(number, z.Config.Phone3CX.CountryPrefix) {
		return "0" + number[len(z.Config.Phone3CX.CountryPrefix):]
	}

	// Apparently the number doesn't start with any of that, so we assume it is already in the correct format
	return number
}

// LogIfErr logs to stderr when an error occurs, doing nothing when err is nil.
func (z *ZammadBridge) LogIfErr(err error, context string) {
	if err == nil {
		return
	}

	log.Error().Err(err).Msg(context)
}
