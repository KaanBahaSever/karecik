package tests

// bcrypt works with at most 72 bytes of a password. It refuses to hash a longer
// one — so register, change-password and reset-password would end in a 500 on
// it — and it compares only the first 72 bytes of whatever it is given, so a
// longer password that starts with the stored one would log in. The API refuses
// such a password with a 422 wherever it would store one, and treats it as a
// wrong password wherever it checks one.

import (
	"net/http"
	"strings"
	"testing"
)

const msgPasswordTooLong = "Şifre çok uzun. En fazla 72 bayt olabilir; Türkçe karakterler 2 bayt sayılır."

func TestPasswordsLongerThan72BytesAreRefused(t *testing.T) {
	mail := &recordingMailer{}
	h := newHarnessWith(t, mail)

	exactly72 := strings.Repeat("a", 72)
	longer := exactly72 + "a"            // 73 bytes that start with exactly72
	turkish36 := strings.Repeat("ş", 36) // 36 characters, 72 bytes
	turkish37 := strings.Repeat("ş", 37) // 37 characters, 74 bytes
	const email = "long-password-owner@example.test"

	errorOf := func(t *testing.T, what string, payload []byte) string {
		t.Helper()
		var body struct {
			Error string `json:"error"`
		}
		decodeInto(t, what, payload, &body)
		return body.Error
	}

	t.Run("register", func(t *testing.T) {
		for _, tc := range []struct{ name, password string }{
			{"73_bytes", longer},
			{"37_turkish_letters", turkish37},
		} {
			resp, payload := h.do(http.MethodPost, "/api/auth/register", "", map[string]any{
				"business_name": "Uzun Şifre Kafe", "email": "register-" + tc.name + "@example.test",
				"password": tc.password,
			})
			expectRefusal(t, "POST /api/auth/register with "+tc.name, resp, payload, msgPasswordTooLong)
		}
	})

	resp, payload := h.do(http.MethodPost, "/api/auth/register", "", map[string]any{
		"business_name": "Tam Sınır Kafe", "email": email, "password": exactly72,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /api/auth/register with a 72-byte password answered %d, want 201: %s", resp.StatusCode, payload)
	}
	session := sessionCookie(resp)
	if session == "" {
		t.Fatalf("fixture: the sign-up set no session cookie")
	}

	login := func(t *testing.T, password string) (int, []byte) {
		t.Helper()
		resp, payload := h.do(http.MethodPost, "/api/auth/login", "", map[string]any{"email": email, "password": password})
		return resp.StatusCode, payload
	}

	t.Run("login", func(t *testing.T) {
		if status, body := login(t, exactly72); status != http.StatusOK {
			t.Fatalf("logging in with the 72-byte password answered %d, want 200: %s", status, body)
		}
		for _, tc := range []struct{ name, password string }{
			{"73_bytes_starting_with_the_password", longer},
			{"37_turkish_letters", turkish37},
		} {
			status, body := login(t, tc.password)
			if status != http.StatusUnauthorized || errorOf(t, "login", body) != "E-posta veya şifre hatalı." {
				t.Errorf("logging in with %s answered %d %s, want the 401 of a wrong password", tc.name, status, body)
			}
		}
	})

	t.Run("change_password", func(t *testing.T) {
		for _, tc := range []struct{ name, password string }{
			{"73_bytes", longer},
			{"37_turkish_letters", turkish37},
		} {
			resp, payload := h.do(http.MethodPost, "/api/auth/change-password", session, map[string]any{
				"current_password": exactly72, "new_password": tc.password,
			})
			expectRefusal(t, "POST /api/auth/change-password to "+tc.name, resp, payload, msgPasswordTooLong)
		}

		resp, payload := h.do(http.MethodPost, "/api/auth/change-password", session, map[string]any{
			"current_password": longer, "new_password": "karecik-new-password",
		})
		if resp.StatusCode != http.StatusUnauthorized || errorOf(t, "change-password", payload) != "Mevcut şifreniz hatalı." {
			t.Errorf("a 73-byte current_password that starts with the password answered %d %s, want the 401 of a "+
				"wrong current password", resp.StatusCode, payload)
		}

		resp, payload = h.do(http.MethodPost, "/api/auth/change-password", session, map[string]any{
			"current_password": exactly72, "new_password": turkish36,
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("changing to a 72-byte password of 36 Turkish letters answered %d, want 200: %s",
				resp.StatusCode, payload)
		}
		if status, body := login(t, turkish36); status != http.StatusOK {
			t.Errorf("logging in with the new 72-byte password answered %d, want 200: %s", status, body)
		}
	})

	t.Run("reset_password", func(t *testing.T) {
		if resp, payload := h.do(http.MethodPost, "/api/auth/forgot-password", "", map[string]any{"email": email}); resp.StatusCode != http.StatusOK {
			t.Fatalf("POST /api/auth/forgot-password answered %d: %s", resp.StatusCode, payload)
		}
		token := tokenFromMail(t, mail.waitForMail(t, 1)[0])

		// Refused before the token is used up: the same link still works after.
		for _, tc := range []struct{ name, password string }{
			{"73_bytes", longer},
			{"37_turkish_letters", turkish37},
		} {
			resp, payload := h.do(http.MethodPost, "/api/auth/reset-password", "", map[string]any{
				"token": token, "password": tc.password,
			})
			expectRefusal(t, "POST /api/auth/reset-password to "+tc.name, resp, payload, msgPasswordTooLong)
		}

		resp, payload := h.do(http.MethodPost, "/api/auth/reset-password", "", map[string]any{
			"token": token, "password": exactly72,
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("resetting to a 72-byte password with the same link answered %d, want 200: %s",
				resp.StatusCode, payload)
		}
		if status, body := login(t, exactly72); status != http.StatusOK {
			t.Errorf("logging in with the reset 72-byte password answered %d, want 200: %s", status, body)
		}
	})
}
