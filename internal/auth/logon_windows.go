//go:build !e2e

package auth

import (
	"errors"
	"fmt"
	"os/user"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var procLogonUserW = windows.NewLazySystemDLL("advapi32.dll").NewProc("LogonUserW")

const (
	logon32LogonNetwork    = 3 // checks the password only: no profile, no desktop session
	logon32ProviderDefault = 0
)

// Windows answers a wrong password at once; wait like PAM's fail delay does,
// so guessing stays slow (the handler checks one login at a time).
const failDelay = 2 * time.Second

// System checks the password with Windows' own logon (LogonUserW) and lets in
// only the account the server runs as, compared by SID, so "name",
// "DOMAIN\name" and "name@domain" all work. Windows Hello (PIN, fingerprint)
// can't be checked this way: it takes the account password — for a Microsoft
// account, its email and online password. The service name is PAM-only.
func System(string) (Authenticator, error) {
	self, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, fmt.Errorf("current user: %w", err)
	}
	owner := self.User.Sid.String()
	current, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("current user: %w", err)
	}
	// A bare name is looked up in the server user's domain (or computer).
	defaultDomain, _, _ := strings.Cut(current.Username, `\`)

	return func(name, password string) error {
		domain := defaultDomain
		if d, n, ok := strings.Cut(name, `\`); ok {
			domain, name = d, n
		} else if strings.Contains(name, "@") {
			domain = "" // a UPN or Microsoft account email carries its domain
		}
		token, err := logonUser(name, domain, password)
		if err != nil {
			time.Sleep(failDelay)
			if isDenied(err) {
				return fmt.Errorf("%w: %v", ErrDenied, err)
			}
			return fmt.Errorf("LogonUser: %w", err)
		}
		defer token.Close()
		tu, err := token.GetTokenUser()
		if err != nil {
			return fmt.Errorf("token user: %w", err)
		}
		if tu.User.Sid.String() != owner {
			time.Sleep(failDelay)
			return ErrDenied
		}
		return nil
	}, nil
}

func logonUser(name, domain, password string) (windows.Token, error) {
	u, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return 0, ErrDenied
	}
	p, err := windows.UTF16PtrFromString(password)
	if err != nil {
		return 0, ErrDenied
	}
	var d *uint16 // nil: name is a UPN
	if domain != "" {
		if d, err = windows.UTF16PtrFromString(domain); err != nil {
			return 0, ErrDenied
		}
	}
	var token windows.Token
	r, _, callErr := procLogonUserW.Call(
		uintptr(unsafe.Pointer(u)),
		uintptr(unsafe.Pointer(d)),
		uintptr(unsafe.Pointer(p)),
		logon32LogonNetwork,
		logon32ProviderDefault,
		uintptr(unsafe.Pointer(&token)),
	)
	if r == 0 {
		return 0, callErr
	}
	return token, nil
}

// isDenied reports the LogonUser errors that mean "this login is refused"
// (wrong password, locked, disabled, expired…) rather than a broken system.
func isDenied(err error) bool {
	for _, code := range []windows.Errno{
		windows.ERROR_LOGON_FAILURE, windows.ERROR_ACCOUNT_RESTRICTION, windows.ERROR_INVALID_LOGON_HOURS,
		windows.ERROR_INVALID_WORKSTATION, windows.ERROR_PASSWORD_EXPIRED, windows.ERROR_ACCOUNT_DISABLED,
		windows.ERROR_LOGON_TYPE_NOT_GRANTED, windows.ERROR_ACCOUNT_EXPIRED, windows.ERROR_PASSWORD_MUST_CHANGE,
		windows.ERROR_ACCOUNT_LOCKED_OUT,
	} {
		if errors.Is(err, code) {
			return true
		}
	}
	return errors.Is(err, ErrDenied)
}
