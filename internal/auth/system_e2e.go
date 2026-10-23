//go:build e2e

package auth

// System in an e2e build (`go run -tags e2e`, playwright.config.js) accepts
// only the fixture login demo / demo-password, so the browser tests never touch
// the machine's real accounts. A normal build never contains this file.
func System(string) (Authenticator, error) {
	return func(name, password string) error {
		if name == "demo" && password == "demo-password" {
			return nil
		}
		return ErrDenied
	}, nil
}
