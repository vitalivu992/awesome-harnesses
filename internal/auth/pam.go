//go:build (linux || darwin) && !e2e

package auth

/*
#cgo LDFLAGS: -lpam
#include <security/pam_appl.h>
#include <stdlib.h>
#include <string.h>

#ifndef PAM_MAX_NUM_MSG
#define PAM_MAX_NUM_MSG 32
#endif

typedef struct {
	const char *user;
	const char *password;
} ah_creds;

// Answers PAM's prompts: the password for hidden ones, the user name for
// visible ones; info and error messages need no answer.
static int ah_conv(int n, const struct pam_message **msg, struct pam_response **resp, void *data) {
	ah_creds *c = data;
	struct pam_response *r;
	int i;

	if (n <= 0 || n > PAM_MAX_NUM_MSG)
		return PAM_CONV_ERR;
	r = calloc(n, sizeof(*r));
	if (r == NULL)
		return PAM_BUF_ERR;
	for (i = 0; i < n; i++) {
		switch (msg[i]->msg_style) {
		case PAM_PROMPT_ECHO_OFF:
			if ((r[i].resp = strdup(c->password)) == NULL)
				goto fail;
			break;
		case PAM_PROMPT_ECHO_ON:
			if ((r[i].resp = strdup(c->user)) == NULL)
				goto fail;
			break;
		case PAM_ERROR_MSG:
		case PAM_TEXT_INFO:
			break;
		default:
			goto fail;
		}
	}
	*resp = r;
	return PAM_SUCCESS;
fail:
	for (i = 0; i < n; i++) {
		if (r[i].resp != NULL) {
			memset(r[i].resp, 0, strlen(r[i].resp));
			free(r[i].resp);
		}
	}
	free(r);
	return PAM_CONV_ERR;
}

// Authenticates user and checks the account (expired, locked) with service.
static int ah_check(const char *service, const char *user, const char *password) {
	ah_creds c = { user, password };
	struct pam_conv conv = { ah_conv, &c };
	pam_handle_t *h = NULL;
	int rc = pam_start(service, user, &conv, &h);
	if (rc != PAM_SUCCESS)
		return rc;
	rc = pam_authenticate(h, PAM_DISALLOW_NULL_AUTHTOK);
	if (rc == PAM_SUCCESS)
		rc = pam_acct_mgmt(h, PAM_DISALLOW_NULL_AUTHTOK);
	pam_end(h, rc);
	return rc;
}

static const char *ah_strerror(int rc) {
	return pam_strerror(NULL, rc);
}
*/
import "C"

import (
	"fmt"
	"os/user"
	"unsafe"
)

// System checks the password with the PAM stack of service (Linux-PAM, or
// OpenPAM on macOS), as a console or desktop login does, and lets in only the
// user the server runs as: the transcripts are that user's files. As a
// non-root process PAM can only check that user's own password anyway.
func System(service string) (Authenticator, error) {
	owner, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("current user: %w", err)
	}
	return func(name, password string) error {
		cService, cName, cPassword := C.CString(service), C.CString(name), C.CString(password)
		defer C.free(unsafe.Pointer(cService))
		defer C.free(unsafe.Pointer(cName))
		defer func() {
			C.memset(unsafe.Pointer(cPassword), 0, C.size_t(len(password)))
			C.free(unsafe.Pointer(cPassword))
		}()

		switch rc := C.ah_check(cService, cName, cPassword); rc {
		case C.PAM_SUCCESS:
		case C.PAM_AUTH_ERR, C.PAM_USER_UNKNOWN, C.PAM_MAXTRIES, C.PAM_ACCT_EXPIRED, C.PAM_NEW_AUTHTOK_REQD,
			C.PAM_PERM_DENIED, C.PAM_CRED_INSUFFICIENT, C.PAM_AUTHTOK_EXPIRED:
			return fmt.Errorf("%w: %s", ErrDenied, C.GoString(C.ah_strerror(rc)))
		default:
			// A broken PAM setup, not a wrong password: keep it for the server log.
			return fmt.Errorf("pam %s: %s", service, C.GoString(C.ah_strerror(rc)))
		}
		// Checked after PAM so a wrong name takes as long as a wrong password.
		if name != owner.Username {
			return ErrDenied
		}
		return nil
	}, nil
}
