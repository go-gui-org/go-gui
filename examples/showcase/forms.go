package main

import (
	"strings"
	"time"

	"github.com/go-gui-org/go-gui/gui"
)

// FormModel holds value fields for the showcase form demo.
// Per-field validation state is managed by the gui.Form runtime.
type FormModel struct {
	Username      string
	Email         string
	AgeText       string
	Password      string
	Confirm       string
	SubmitMessage string
	AgeValue      gui.Opt[float64]
}

const showcaseFormID = "showcase-form"

// --- Sync validators ---

func validateUsernameFormSync(
	f gui.FormFieldSnapshot, _ gui.FormSnapshot,
) []gui.FormIssue {
	v := strings.TrimSpace(f.Value)
	if v == "" {
		return []gui.FormIssue{{Msg: "username required"}}
	}
	if len(v) < 3 {
		return []gui.FormIssue{{Msg: "username min length is 3"}}
	}
	return nil
}

func validateEmailFormSync(
	f gui.FormFieldSnapshot, _ gui.FormSnapshot,
) []gui.FormIssue {
	v := strings.TrimSpace(f.Value)
	if v == "" {
		return []gui.FormIssue{{Msg: "email required"}}
	}
	if !strings.Contains(v, "@") {
		return []gui.FormIssue{{Msg: "email must contain @"}}
	}
	return nil
}

func validateAgeFormSync(
	f gui.FormFieldSnapshot, _ gui.FormSnapshot,
) []gui.FormIssue {
	if strings.TrimSpace(f.Value) == "" {
		return []gui.FormIssue{{Msg: "age required"}}
	}
	return nil
}

func validatePasswordFormSync(
	f gui.FormFieldSnapshot, _ gui.FormSnapshot,
) []gui.FormIssue {
	if len(f.Value) < 8 {
		return []gui.FormIssue{{Msg: "password min length is 8"}}
	}
	return nil
}

// validateConfirmFormSync reads the password through the form
// snapshot. It is registered on the confirm field only: when the
// password changes, the form re-checks confirm by itself.
func validateConfirmFormSync(
	f gui.FormFieldSnapshot, form gui.FormSnapshot,
) []gui.FormIssue {
	if f.Value != form.Values["password"] {
		return []gui.FormIssue{{Msg: "passwords do not match"}}
	}
	return nil
}

// --- Async validator ---

func validateUsernameFormAsync(
	f gui.FormFieldSnapshot, _ gui.FormSnapshot,
	signal *gui.GridAbortSignal,
) []gui.FormIssue {
	// Simulate network delay.
	for range 4 {
		if signal.IsAborted() {
			return nil
		}
		time.Sleep(60 * time.Millisecond)
	}
	if signal.IsAborted() {
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(f.Value)) {
	case "admin", "root", "system":
		return []gui.FormIssue{{Msg: "username already taken"}}
	}
	return nil
}

// --- Reusable adapter configs ---

var (
	usernameSyncVals  = []gui.FormSyncValidator{validateUsernameFormSync}
	usernameAsyncVals = []gui.FormAsyncValidator{validateUsernameFormAsync}
	emailSyncVals     = []gui.FormSyncValidator{validateEmailFormSync}
	ageSyncVals       = []gui.FormSyncValidator{validateAgeFormSync}
	passwordSyncVals  = []gui.FormSyncValidator{validatePasswordFormSync}
	confirmSyncVals   = []gui.FormSyncValidator{validateConfirmFormSync}
)

func usernameAdapterCfg(value string) gui.FormFieldAdapterCfg {
	return gui.FormFieldAdapterCfg{
		FieldID:         "username",
		Value:           value,
		SyncValidators:  usernameSyncVals,
		AsyncValidators: usernameAsyncVals,
	}
}

func emailAdapterCfg(value string) gui.FormFieldAdapterCfg {
	return gui.FormFieldAdapterCfg{
		FieldID:        "email",
		Value:          value,
		SyncValidators: emailSyncVals,
	}
}

func ageAdapterCfg(value string) gui.FormFieldAdapterCfg {
	return gui.FormFieldAdapterCfg{
		FieldID:        "age",
		Value:          value,
		SyncValidators: ageSyncVals,
	}
}

func passwordAdapterCfg(value string) gui.FormFieldAdapterCfg {
	return gui.FormFieldAdapterCfg{
		FieldID:        "password",
		Value:          value,
		SyncValidators: passwordSyncVals,
	}
}

func confirmAdapterCfg(value string) gui.FormFieldAdapterCfg {
	return gui.FormFieldAdapterCfg{
		FieldID:        "confirm",
		Value:          value,
		SyncValidators: confirmSyncVals,
	}
}
