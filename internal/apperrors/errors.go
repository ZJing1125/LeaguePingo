// Package apperrors defines the shared, closed vocabulary for user-facing errors.
// Never retain arbitrary error text, response bodies, URLs or credentials here.
package apperrors

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"net"
	"net/http"
)

type Code string

const (
	Unexpected                 Code = "Unexpected"
	ClientNotRunning           Code = "ClientNotRunning"
	ClientUnavailable          Code = "ClientUnavailable"
	ClientPermissionDenied     Code = "ClientPermissionDenied"
	ClientConfigurationInvalid Code = "ClientConfigurationInvalid"
	NotLoggedIn                Code = "NotLoggedIn"
	AccessDenied               Code = "AccessDenied"
	ServiceBusy                Code = "ServiceBusy"
	RateLimited                Code = "RateLimited"
	RequestTimeout             Code = "RequestTimeout"
	RequestCanceled            Code = "RequestCanceled"
	NetworkUnavailable         Code = "NetworkUnavailable"
	DataUnavailable            Code = "DataUnavailable"
	InvalidResponse            Code = "InvalidResponse"
	Unsupported                Code = "Unsupported"
	StaleData                  Code = "StaleData"
	PlayerIdentityUnavailable  Code = "PlayerIdentityUnavailable"
	PlayerNotFound             Code = "PlayerNotFound"
	InvalidRiotID              Code = "InvalidRiotID"
	QueryBusy                  Code = "QueryBusy"
	NoRoster                   Code = "NoRoster"
	InvalidPage                Code = "InvalidPage"
	HistoryPageInvalid         Code = "HistoryPageInvalid"
	HistoryRepeated            Code = "HistoryRepeated"
	InvalidMatch               Code = "InvalidMatch"
	InvalidQueue               Code = "InvalidQueue"
	InvalidPosition            Code = "InvalidPosition"
	InvalidCandidates          Code = "InvalidCandidates"
	GuideUnavailable           Code = "GuideUnavailable"
	SettingsReadFailed         Code = "SettingsReadFailed"
	SettingsWriteFailed        Code = "SettingsWriteFailed"
	InvalidSidebarWidth        Code = "InvalidSidebarWidth"
	InvalidHoldKey             Code = "InvalidHoldKey"
	InvalidShortcut            Code = "InvalidShortcut"
	ShortcutUnavailable        Code = "ShortcutUnavailable"
	HoldUnavailable            Code = "HoldUnavailable"
	OverlayUnavailable         Code = "OverlayUnavailable"
	AppNotReady                Code = "AppNotReady"
	DiagnosticWriteFailed      Code = "DiagnosticWriteFailed"
	InvalidAcceptDelay         Code = "InvalidAcceptDelay"
	InvalidQueueDelay          Code = "InvalidQueueDelay"
	MatchNotReady              Code = "MatchNotReady"
)

type Service string

const (
	LocalClient  Service = "client"
	DataService  Service = "data"
	GuideService Service = "guide"
	Desktop      Service = "desktop"
)

//go:embed messages.json
var messageJSON []byte
var messages = func() map[Code]string {
	var values map[Code]string
	if err := json.Unmarshal(messageJSON, &values); err != nil {
		panic("invalid error message catalog")
	}
	return values
}()

type Error struct {
	code   Code
	status int
	// Only context sentinels may be retained, never raw transport/parser errors.
	cause error
}

type Presentation struct {
	Code    Code   `json:"code"`
	Message string `json:"message"`
}

func Message(code Code) string {
	if text, ok := messages[code]; ok {
		return text
	}
	return messages[Unexpected]
}
func New(code Code) *Error {
	if _, ok := messages[code]; !ok {
		code = Unexpected
	}
	return &Error{code: code}
}
func (e *Error) Error() string { return Message(e.code) }
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}
func (e *Error) MarshalJSON() ([]byte, error) {
	return json.Marshal(Presentation{e.code, e.Error()})
}

// Normalize preserves known classifications through wrapping but drops the
// wrapper's text. Unknown errors are classified by type, never substring.
func Normalize(err error, fallback Code) *Error {
	if err == nil {
		return nil
	}
	var known *Error
	if errors.As(err, &known) {
		if known == nil {
			return New(fallback)
		}
		copy := *known
		return &copy
	}
	if errors.Is(err, context.Canceled) {
		e := New(RequestCanceled)
		e.cause = context.Canceled
		return e
	}
	var timeout net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &timeout) && timeout.Timeout()) {
		e := New(RequestTimeout)
		e.cause = context.DeadlineExceeded
		return e
	}
	var syntax *json.SyntaxError
	var mismatch *json.UnmarshalTypeError
	if errors.As(err, &syntax) || errors.As(err, &mismatch) {
		return New(InvalidResponse)
	}
	return New(fallback)
}
func CodeOf(err error) Code {
	if err == nil {
		return ""
	}
	return Normalize(err, Unexpected).code
}
func UserMessage(err error, fallback Code) string {
	if err == nil {
		return ""
	}
	return Normalize(err, fallback).Error()
}

// Present guards the entire Wails service return, including its top-level
// RuntimeError.message (MarshalError alone only controls RuntimeError.cause).
func Present(err *error) {
	if *err != nil {
		*err = Normalize(*err, Unexpected)
		Failures.Record(Desktop, *err)
	}
}
func Marshal(err error) []byte {
	e := Normalize(err, Unexpected)
	if e == nil {
		return []byte("null")
	}
	data, _ := e.MarshalJSON() // Static strings and fixed code: encoding cannot fail.
	return data
}
func Request(ctx context.Context, err error, service Service) *Error {
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	fallback := NetworkUnavailable
	if service == LocalClient {
		fallback = ClientUnavailable
	}
	e := Normalize(err, fallback)
	return e
}
func HTTP(service Service, status int) *Error {
	code := DataUnavailable
	switch {
	case status == http.StatusUnauthorized:
		code = NotLoggedIn
	case status == http.StatusForbidden:
		code = AccessDenied
	case status == http.StatusTooManyRequests:
		code = RateLimited
	case status == http.StatusRequestTimeout || status == http.StatusGatewayTimeout:
		code = RequestTimeout
	case status >= 500 && status <= 599:
		code = ServiceBusy
	}
	if service == GuideService && (code == NotLoggedIn || code == AccessDenied) {
		code = GuideUnavailable
	}
	e := New(code)
	if status >= 100 && status <= 599 {
		e.status = status
	}
	return e
}
